package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// All uncertain Rex method names and request shapes are kept here for sandbox adjustment.
type probeSpec struct {
	service, method string
	body            func(map[string]string) any
}

var probeMethods = map[string]probeSpec{
	"accounts": {"UserProfile", "getAccessibleAccounts", func(_ map[string]string) any { return map[string]any{} }},
	"path":     {"Listings", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"tags": {"AdminTags", "search", func(v map[string]string) any {
		return map[string]any{"criteria": []any{map[string]any{"name": v["tag"]}}, "limit": 2}
	}},
	"listings": {"Listings", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"users":    {"AccountUsers", "search", func(_ map[string]string) any { return map[string]any{"limit": 2} }},
	"contactCreate": {"Contacts", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"name": v["name"], "related": map[string]any{"contact_tags": []any{v["tag"]}}}, "return_id": true}
	}},
	"leadAssigned": {"Leads", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"listing": map[string]any{"id": v["listing"]}, "assignee": map[string]any{"id": v["user"]}, "notes": v["name"] + " lead assigned"}, "return_id": true}
	}},
	"leadListing": {"Leads", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"listing": map[string]any{"id": v["listing"]}, "notes": v["name"] + " lead listing"}, "return_id": true}
	}},
	"leadRead": {"Leads", "read", func(v map[string]string) any { return map[string]any{"id": v["lead"]} }},
	"reminderCreate": {"Reminders", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"remindee": map[string]any{"id": v["remindee"]}, "description": "AXLEO TEST - push check"}, "return_id": true}
	}},
	"events": {"ListingEvents", "search", func(v map[string]string) any {
		return map[string]any{"criteria": []any{map[string]any{"listing_id": v["listing"], "event_type": "open home"}}, "limit": 2}
	}},
	"eventsFields":   {"ListingEvents", "describeSearchFields", func(_ map[string]string) any { return map[string]any{} }},
	"attendeeFields": {"ListingEventAttendees", "describeModel", func(_ map[string]string) any { return map[string]any{} }},
	"letter": {"Contacts", "search", func(_ map[string]string) any {
		return map[string]any{"criteria": []any{map[string]any{"contact_by_letter": 0}}, "limit": 1}
	}},
	"contactFields": {"Contacts", "describeSearchFields", func(_ map[string]string) any { return map[string]any{} }},
	"propertyCreate": {"Properties", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"address": v["name"], "related": map[string]any{"property_tags": []any{v["tag"]}}}, "return_id": true}
	}},
	"propertyRead": {"Properties", "read", func(v map[string]string) any {
		return map[string]any{"id": v["property"], "extra_fields": []string{"related", "owners", "tenants"}}
	}},
	"properties": {"Properties", "search", func(_ map[string]string) any {
		return map[string]any{"limit": 3, "extra_fields": []string{"related", "owners", "tenants"}}
	}},
	"remindees":  {"Reminders", "getRemindeeList", func(_ map[string]string) any { return map[string]any{} }},
	"dedupe":     {"Dedupe", "findPossibleDuplicates", func(v map[string]string) any { return map[string]any{"record_type": "contacts", "id": v["contact"]} }},
	"leads":      {"Leads", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"reminders":  {"Reminders", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"matches":    {"MatchProfiles", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"oabs":       {"Oabs", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"categories": {"SystemValues", "getValueCategories", func(_ map[string]string) any { return map[string]any{} }},
	"values":     {"SystemValues", "getCategoryValues", func(v map[string]string) any { return map[string]any{"category": v["category"]} }},
}
var probeS1 = []string{"remindees", "dedupe", "tags", "leads", "reminders", "matches", "oabs", "events", "categories", "values"}

type probeRecord struct {
	Service string `json:"service"`
	ID      string `json:"id"`
}
type probeState struct {
	Records []probeRecord `json:"records"`
}
type probeRow struct {
	Check, Verdict, Conclusion string
	Files                      []string
}
type probeRun struct {
	c      *rexClient
	dir    string
	rows   []probeRow
	counts map[string]int
	state  probeState
	vars   map[string]string
	write  bool
}

func probeID(raw json.RawMessage) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	if m, ok := v.(map[string]any); ok {
		if id, ok := m["id"]; ok {
			return fmt.Sprint(id)
		}
		if r, ok := m["result"]; ok {
			return probeIDValue(r)
		}
	}
	return probeIDValue(v)
}
func probeIDValue(v any) string {
	if m, ok := v.(map[string]any); ok {
		if id, ok := m["id"]; ok {
			return fmt.Sprint(id)
		}
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprint(x)
	}
	return ""
}
func probeFirst(raw json.RawMessage) string {
	var v any
	_ = json.Unmarshal(raw, &v)
	if m, ok := v.(map[string]any); ok {
		v = m["result"]
	}
	if m, ok := v.(map[string]any); ok {
		for _, k := range []string{"rows", "items", "data", "results"} {
			if a, ok := m[k].([]any); ok {
				v = a
				break
			}
		}
	}
	if a, ok := v.([]any); ok && len(a) > 0 {
		return probeIDValue(a[0])
	}
	return ""
}
func probeTree(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := map[string]any{}
		for k, y := range x {
			m[k] = probeTree(y)
		}
		return m
	case []any:
		if len(x) == 0 {
			return []any{}
		}
		return []any{probeTree(x[0])}
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	default:
		return "null"
	}
}

// Raw files retain the shape but never retain real sandbox values; only our test prefix is preserved.
func probeMask(v any, secrets ...string) any {
	switch x := v.(type) {
	case map[string]any:
		m := map[string]any{}
		for k, y := range x {
			if k == "password" || k == "token" || k == "Authorization" || k == "email" {
				m[k] = "[REDACTED]"
			} else {
				m[k] = probeMask(y, secrets...)
			}
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, y := range x {
			a[i] = probeMask(y, secrets...)
		}
		return a
	case string:
		for _, s := range secrets {
			if s != "" && strings.Contains(x, s) {
				return "[REDACTED]"
			}
		}
		if strings.HasPrefix(x, "AXLEO TEST - ") {
			return x
		}
		return "[MASKED]"
	default:
		return x
	}
}
func (p *probeRun) save(check, key string, request, raw any, err error) string {
	p.counts[check]++
	file := fmt.Sprintf("raw/%s-%d.json", check, p.counts[check])
	var v any
	if b, ok := raw.([]byte); ok {
		_ = json.Unmarshal(b, &v)
	}
	doc := map[string]any{"method": key, "request": probeMask(request, p.c.Token, os.Getenv("REX_PASSWORD")), "response": probeMask(v, p.c.Token, os.Getenv("REX_PASSWORD")), "key_tree": probeTree(v)}
	if err != nil {
		doc["outcome"] = "ERROR (see verdict)"
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	_ = os.WriteFile(filepath.Join(p.dir, file), b, 0600)
	return file
}
func (p *probeRun) call(ctx context.Context, check, key string) (json.RawMessage, error) {
	spec := probeMethods[key]
	body := spec.body(p.vars)
	raw, _, err := p.c.Call(ctx, spec.service, spec.method, body)
	p.vars["lastFile"] = p.save(check, key, body, raw, err)
	return raw, err
}
func (p *probeRun) row(check, verdict, conclusion string, files ...string) {
	p.rows = append(p.rows, probeRow{check, verdict, conclusion, files})
}
func (p *probeRun) created(service string, raw json.RawMessage) error {
	id := probeID(raw)
	if id == "" {
		return errors.New("创建成功但未返回 ID；请人工排查，不能安全自动清理")
	}
	p.state.Records = append(p.state.Records, probeRecord{service, id})
	return p.saveState()
}
func (p *probeRun) saveState() error {
	b, err := json.MarshalIndent(p.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.dir, "state.json"), b, 0600)
}
func (p *probeRun) runCheck(ctx context.Context, id string) {
	files := []string{}
	call := func(key string) (json.RawMessage, error) {
		raw, err := p.call(ctx, id, key)
		files = append(files, p.vars["lastFile"])
		return raw, err
	}
	outcome := func(err error, yes string) {
		if err != nil {
			p.row(id, "UNKNOWN", "接口拒绝或参数待核实："+probeError(err, p.c), files...)
		} else {
			p.row(id, "PASS", yes, files...)
		}
	}
	switch id {
	case "P0":
		_, e := call("accounts")
		spec := probeMethods["path"]
		body := spec.body(p.vars)
		low, eLow := call("path")
		_ = low
		raw, _, eCamel := p.c.callPath(ctx, spec.service, spec.method, body, true)
		files = append(files, p.save(id, "path-CamelCase", body, raw, eCamel))
		if e != nil {
			outcome(e, "账户已可读取")
			return
		}
		p.row(id, "PASS", fmt.Sprintf("登录和账户读取成功；小写连字符=%v，CamelCase=%v（失败原因见原始文件）", eLow == nil, eCamel == nil), files...)
	case "F1":
		raw, err := call("contactCreate")
		if err == nil {
			err = p.created("Contacts", raw)
			p.vars["contact"] = probeID(raw)
		}
		if err != nil {
			outcome(err, "")
			return
		}
		tags, tagErr := call("tags")
		if tagErr != nil {
			outcome(tagErr, "")
		} else if strings.Contains(string(tags), p.vars["tag"]) {
			p.row(id, "PASS", "新标签在 AdminTags 搜索中出现：自动创建", files...)
		} else {
			p.row(id, "UNKNOWN", "联系人创建成功，但搜索未找到新标签；需核对搜索条件或标签创建规则", files...)
		}
	case "F2":
		l, el := call("listings")
		u, eu := call("users")
		p.vars["listing"] = probeFirst(l)
		p.vars["user"] = probeFirst(u)
		if el != nil || eu != nil || p.vars["listing"] == "" || p.vars["user"] == "" {
			p.row(id, "UNKNOWN", "未找到挂牌或其他账号用户，或搜索参数不被接受", files...)
			return
		}
		a, ea := call("leadAssigned")
		if ea == nil {
			ea = p.created("Leads", a)
		}
		b, eb := call("leadListing")
		if eb == nil {
			eb = p.created("Leads", b)
			p.vars["lead"] = probeID(b)
		}
		var read json.RawMessage
		if eb == nil {
			read, eb = call("leadRead")
		}
		if ea != nil || eb != nil {
			p.row(id, "UNKNOWN", "指定负责人或自动分配请求失败："+probeError(errors.Join(ea, eb), p.c), files...)
		} else {
			var envelope map[string]any
			_ = json.Unmarshal(read, &envelope)
			assignee := "未返回 assignee 字段"
			if result, ok := envelope["result"].(map[string]any); ok {
				if a, ok := result["assignee"].(map[string]any); ok && a["id"] != nil {
					assignee = "已返回 assignee（是否等于搜索到的用户：" + fmt.Sprint(fmt.Sprint(a["id"]) == p.vars["user"]) + "）"
				}
			}
			p.row(id, "PASS", "两种线索已创建；listing-only 读回："+assignee, files...)
		}
	case "F3":
		if p.vars["remindee"] == "" {
			p.row(id, "MANUAL", "未指定 --remindee，未创建提醒；提供用户 ID 后请人工查看 Rex 手机推送")
			return
		}
		raw, err := call("reminderCreate")
		if err == nil {
			err = p.created("Reminders", raw)
		}
		if err != nil {
			outcome(err, "")
			return
		}
		p.row(id, "MANUAL", "已创建提醒；请人工查看该用户 Rex 手机端是否收到推送", files...)
	case "F4":
		if p.vars["listing"] == "" {
			l, _ := call("listings")
			p.vars["listing"] = probeFirst(l)
		}
		_, e := call("events")
		_, e1 := call("eventsFields")
		_, e2 := call("attendeeFields")
		p.row(id, "UNKNOWN", fmt.Sprintf("场次搜索/字段接口结果见原始文件（错误：%s）；参会者写入能力未确认；vendor report needs manual check in Rex web", probeError(errors.Join(e, e1, e2), p.c)), files...)
	case "F5":
		_, e := call("letter")
		_, e2 := call("contactFields")
		if p.write {
			raw, ce := call("propertyCreate")
			if ce == nil {
				ce = p.created("Properties", raw)
				p.vars["property"] = probeID(raw)
			}
			if ce == nil {
				_, ce = call("propertyRead")
			}
			e = errors.Join(e, ce)
		}
		outcome(errors.Join(e, e2), "联系人按不投信字段搜索、字段列表及自建房产标签的读回结果见原始文件")
	case "F7":
		raw, err := call("properties")
		p.vars["property"] = probeFirst(raw)
		if p.vars["property"] != "" {
			_, e := call("propertyRead")
			err = errors.Join(err, e)
		}
		outcome(err, "房产 search/read 的业主、租客关系字段见 key_tree")
	case "S1":
		failures := 0
		for _, key := range probeS1 {
			if key == "dedupe" && p.vars["contact"] == "" {
				continue
			}
			if key == "values" && p.vars["category"] == "" {
				continue
			}
			raw, err := call(key)
			if err != nil {
				failures++
			}
			if key == "categories" {
				p.vars["category"] = probeFirst(raw)
			}
		}
		verdict := "PASS"
		if failures > 0 {
			verdict = "UNKNOWN"
		}
		p.row(id, verdict, fmt.Sprintf("返回字段类型树已保存；%d 个接口待核实；无测试联系人时跳过去重", failures), files...)
	}
}
func probeError(err error, c *rexClient) string {
	if err == nil {
		return "无"
	}
	s := err.Error()
	for _, secret := range []string{c.Token, os.Getenv("REX_PASSWORD"), os.Getenv("REX_EMAIL")} {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[REDACTED]")
		}
	}
	s = regexp.MustCompile(`[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}`).ReplaceAllString(s, "[REDACTED EMAIL]")
	s = regexp.MustCompile(`\b(?:\+?\d[\d ()-]{7,}\d)\b`).ReplaceAllString(s, "[REDACTED PHONE]")
	return strings.ReplaceAll(s, "|", "/")
}
func (p *probeRun) report() error {
	var b strings.Builder
	b.WriteString("# Rex 沙箱验证报告\n\n| 检查 | 结论 | 说明 | 原始文件 |\n|---|---|---|---|\n")
	for _, r := range p.rows {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", r.Check, r.Verdict, strings.ReplaceAll(r.Conclusion, "|", "/"), strings.Join(r.Files, ", ")))
	}
	return os.WriteFile(filepath.Join(p.dir, "report.md"), []byte(b.String()), 0600)
}
func probeCleanup(ctx context.Context, c *rexClient, file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var state probeState
	if err = json.Unmarshal(b, &state); err != nil {
		return err
	}
	var failures []error
	for i := len(state.Records) - 1; i >= 0; i-- {
		r := state.Records[i]
		_, _, err = c.Call(ctx, r.Service, "delete", map[string]any{"id": r.ID})
		if err != nil {
			_, _, err = c.Call(ctx, r.Service, "archive", map[string]any{"id": r.ID})
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s id %s: %w", r.Service, r.ID, err))
		} else {
			state.Records = append(state.Records[:i], state.Records[i+1:]...)
			data, _ := json.MarshalIndent(state, "", "  ")
			if e := os.WriteFile(file, data, 0600); e != nil {
				return e
			}
		}
	}
	return errors.Join(failures...)
}
func rexProbe(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	base := env("REX_BASE_URL", "https://api.rexsoftware.com")
	c := newRexClient(base, env("REX_APP_ID", "Integration:PlaceSunnybank:Portal"))
	if ms, err := strconv.Atoi(os.Getenv("REX_INTERVAL_MS")); err == nil && ms >= 0 {
		c.Interval = time.Duration(ms) * time.Millisecond
	}
	email, password, account := os.Getenv("REX_EMAIL"), os.Getenv("REX_PASSWORD"), os.Getenv("REX_ACCOUNT_ID")
	if len(args) > 0 && args[0] == "cleanup" {
		f := flag.NewFlagSet("rex-probe cleanup", flag.ContinueOnError)
		state := f.String("state", "", "state.json path")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if *state == "" {
			return errors.New("--state required")
		}
		if email == "" || password == "" {
			return errors.New("REX_EMAIL and REX_PASSWORD required")
		}
		if _, err := c.Login(ctx, email, password, account); err != nil {
			return err
		}
		return probeCleanup(ctx, c, *state)
	}
	f := flag.NewFlagSet("rex-probe", flag.ContinueOnError)
	confirmed := f.String("account", "", "explicit sandbox account ID")
	write := f.Bool("write", false, "enable test writes")
	only := f.String("only", "", "comma-separated checks")
	out := f.String("out", "", "report directory")
	remindee := f.String("remindee", "", "Rex user ID")
	keep := f.Bool("keep", false, "leave test records for inspection")
	if err := f.Parse(args); err != nil {
		return err
	}
	if len(f.Args()) > 0 {
		return errors.New("unexpected positional argument")
	}
	if email == "" || password == "" {
		return errors.New("REX_EMAIL and REX_PASSWORD required")
	}
	// First login without selecting an account, to inspect accessible accounts on mismatch.
	if *confirmed == "" || account == "" || *confirmed != account {
		if _, err := c.Login(ctx, email, password, ""); err != nil {
			return err
		}
		raw, _, err := c.Call(ctx, "UserProfile", "getAccessibleAccounts", map[string]any{})
		if err == nil {
			var v any
			_ = json.Unmarshal(raw, &v)
			fmt.Fprintln(os.Stderr, "Accessible accounts (IDs only):", probeAccountIDs(v))
		} else {
			fmt.Fprintln(os.Stderr, "Accessible accounts unavailable:", probeError(err, c))
		}
		return errors.New("--account must match REX_ACCOUNT_ID; no checks or writes performed")
	}
	if *out == "" {
		*out = fmt.Sprintf("rex-probe-%s", time.Now().UTC().Format("20060102-150405"))
	}
	if err := os.MkdirAll(filepath.Join(*out, "raw"), 0700); err != nil {
		return err
	}
	p := &probeRun{c: c, dir: *out, counts: map[string]int{}, vars: map[string]string{"name": "AXLEO TEST - " + time.Now().UTC().Format("20060102-150405"), "remindee": *remindee}, write: *write}
	p.vars["tag"] = "AXLEO TEST - tag-" + time.Now().UTC().Format("20060102-150405")
	defer func() { _ = p.report() }()
	raw, err := c.Login(ctx, email, password, account)
	p.save("P0", "login", map[string]any{"email": email, "password": password, "account_id": account}, raw, err)
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	if *only != "" {
		for _, id := range strings.Split(*only, ",") {
			selected[strings.TrimSpace(id)] = true
		}
		for id := range selected {
			if !strings.Contains("|P0|F1|F2|F3|F4|F5|F7|S1|", "|"+id+"|") {
				return fmt.Errorf("unknown check %s", id)
			}
		}
	}
	for _, id := range []string{"P0", "F1", "F2", "F3", "F4", "F5", "F7", "S1"} {
		if len(selected) > 0 && !selected[id] {
			continue
		}
		if !*write && (id == "F1" || id == "F2" || id == "F3") {
			p.row(id, "UNKNOWN", "只读模式：跳过写入检查")
			continue
		}
		p.runCheck(ctx, id)
	}
	if len(p.state.Records) > 0 && !*keep {
		if err := probeCleanup(ctx, c, filepath.Join(*out, "state.json")); err != nil {
			p.row("cleanup", "FAIL", "自动清理未完成；请用 cleanup --state 重试："+probeError(err, c))
			return err
		}
		p.row("cleanup", "PASS", "测试记录已删除或归档")
	}
	return p.report()
}
func probeAccountIDs(v any) []string {
	var ids []string
	var walk func(any)
	walk = func(x any) {
		switch y := x.(type) {
		case map[string]any:
			for k, z := range y {
				if k == "id" || k == "account_id" {
					ids = append(ids, fmt.Sprint(z))
				} else {
					walk(z)
				}
			}
		case []any:
			for _, z := range y {
				walk(z)
			}
		}
	}
	walk(v)
	sort.Strings(ids)
	return ids
}
