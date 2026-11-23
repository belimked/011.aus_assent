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
		// Verified in sandbox 2026-11-17: criteria are {name: <search field>, value}; AdminTags fields: id, object_type, tag, cached_count.
		return map[string]any{"criteria": []any{map[string]any{"name": "tag", "value": v["tag"]}}, "limit": 2}
	}},
	"listings": {"Listings", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"users":    {"AccountUsers", "search", func(_ map[string]string) any { return map[string]any{"limit": 2} }},
	// Write shapes verified against existing sandbox records 2026-11-17 (names/tags live under related.*).
	"contactCreate": {"Contacts", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"type": "person", "related": map[string]any{
			"contact_names": []any{map[string]any{"name_first": v["name"], "name_last": "AXLEO TEST - probe"}},
			"contact_tags":  []any{map[string]any{"tag": v["tag"]}},
		}}, "return_id": true}
	}},
	"leadAssigned": {"Leads", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"listing": map[string]any{"id": v["listing"]}, "assignee": map[string]any{"id": v["user"]}, "note": v["name"] + " lead assigned"}, "return_id": true}
	}},
	"leadListing": {"Leads", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"listing": map[string]any{"id": v["listing"]}, "note": v["name"] + " lead listing"}, "return_id": true}
	}},
	"leadRead":    {"Leads", "read", func(v map[string]string) any { return map[string]any{"id": v["lead"]} }},
	"contactRead": {"Contacts", "read", func(v map[string]string) any { return map[string]any{"id": v["contact"]} }},
	"reminderCreate": {"Reminders", "create", func(v map[string]string) any {
		// Rex requires the reminder to be attached to a record; use this run's test contact (F1 runs first).
		return map[string]any{"data": map[string]any{"remindee": map[string]any{"id": v["remindee"], "type": "user"}, "contact": map[string]any{"id": v["contact"]}, "reminder_type": map[string]any{"id": "task"}, "reminder_description": "AXLEO TEST - push check", "reminder_date": time.Now().Add(24 * time.Hour).Format("2006-01-02")}, "return_id": true}
	}},
	"events": {"ListingEvents", "search", func(v map[string]string) any {
		return map[string]any{"criteria": []any{map[string]any{"name": "listing_id", "value": v["listing"]}}, "limit": 2}
	}},
	"eventsFields": {"ListingEvents", "describeSearchFields", func(_ map[string]string) any { return map[string]any{} }},
	// There is no ListingEventAttendees class; Rex exposes OpenHomeCheckIns (read, checkIn) instead.
	"attendeeFields": {"OpenHomeCheckIns", "describe", func(_ map[string]string) any { return map[string]any{"include_detail": true} }},
	"letter": {"Contacts", "search", func(_ map[string]string) any {
		return map[string]any{"criteria": []any{map[string]any{"name": "contact.contact_by_letter", "value": "0"}}, "limit": 1, "result_format": "ids"}
	}},
	// Property-level Do Not Contact (search field property.is_dnd, real field).
	"propertyDnd": {"Properties", "search", func(_ map[string]string) any {
		return map[string]any{"criteria": []any{map[string]any{"name": "property.is_dnd", "value": "1"}}, "limit": 1, "result_format": "ids"}
	}},
	"contactFields": {"Contacts", "describeSearchFields", func(_ map[string]string) any { return map[string]any{} }},
	"propertyCreate": {"Properties", "create", func(v map[string]string) any {
		return map[string]any{"data": map[string]any{"property_category": map[string]any{"id": "residential"}, "adr_street_number": "1", "adr_street_name": v["name"], "adr_suburb_or_town": "Sunnybank", "adr_state_or_region": "QLD", "adr_postcode": "4109",
			"related": map[string]any{"property_tags": []any{map[string]any{"tag": v["tag"]}}}}, "return_id": true}
	}},
	"propertyRead": {"Properties", "read", func(v map[string]string) any {
		return map[string]any{"id": v["property"]}
	}},
	// Owner/tenant links: search extra_options.extra_fields contact_reln_property -> related.contact_reln_property[{contact, reln_type, do_not_contact}].
	"properties": {"Properties", "search", func(_ map[string]string) any {
		return map[string]any{"limit": 3, "extra_options": map[string]any{"extra_fields": []string{"contact_reln_property"}}}
	}},
	"remindees": {"Reminders", "getRemindeeList", func(_ map[string]string) any { return map[string]any{} }},
	"dedupe": {"Dedupe", "findPossibleDuplicates", func(v map[string]string) any {
		return map[string]any{"service_name": "Contacts", "record_id": v["contact"], "match_fields": []string{"name", "email", "phone"}}
	}},
	"leads":      {"Leads", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"reminders":  {"Reminders", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"matches":    {"MatchProfiles", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"oabs":       {"Oabs", "search", func(_ map[string]string) any { return map[string]any{"limit": 1} }},
	"categories": {"SystemValues", "getValueCategories", func(_ map[string]string) any { return map[string]any{} }},
	"values":     {"SystemValues", "getCategoryValues", func(v map[string]string) any { return map[string]any{"list_name": v["category"]} }},
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
			return probeIDValue(id)
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
			return probeIDValue(id)
		}
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
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
	switch b := raw.(type) {
	case json.RawMessage:
		_ = json.Unmarshal(b, &v)
	case []byte:
		_ = json.Unmarshal(b, &v)
	}
	doc := map[string]any{"method": key, "request": probeMask(request, p.c.Token, p.c.Password), "response": probeMask(v, p.c.Token, p.c.Password), "key_tree": probeTree(v)}
	if err != nil {
		doc["outcome"] = "ERROR: " + err.Error()
		// Rex error objects describe the API, not records; keep them readable (secrets still redacted).
		if m, ok := v.(map[string]any); ok && m["error"] != nil {
			doc["error"] = probeRedact(m["error"], p.c.Token, p.c.Password)
		}
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	_ = os.WriteFile(filepath.Join(p.dir, file), b, 0600)
	return file
}
func probeRedact(v any, secrets ...string) any {
	switch x := v.(type) {
	case map[string]any:
		m := map[string]any{}
		for k, y := range x {
			m[k] = probeRedact(y, secrets...)
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, y := range x {
			a[i] = probeRedact(y, secrets...)
		}
		return a
	case string:
		for _, s := range secrets {
			if s != "" && strings.Contains(x, s) {
				x = strings.ReplaceAll(x, s, "[REDACTED]")
			}
		}
		return x
	}
	return v
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
		read, readErr := call("contactRead")
		tags, tagErr := call("tags")
		var env struct {
			Result struct {
				Related struct {
					Tags []struct {
						Tag string `json:"tag"`
					} `json:"contact_tags"`
				} `json:"related"`
			} `json:"result"`
		}
		_ = json.Unmarshal(read, &env)
		onContact := false
		for _, t := range env.Result.Related.Tags {
			onContact = onContact || t.Tag == p.vars["tag"]
		}
		var tagEnv struct {
			Result struct {
				Total any `json:"total"`
			} `json:"result"`
		}
		_ = json.Unmarshal(tags, &tagEnv)
		inAdmin := fmt.Sprint(tagEnv.Result.Total) != "0" && tagEnv.Result.Total != nil
		if readErr != nil || tagErr != nil {
			outcome(errors.Join(readErr, tagErr), "")
		} else if onContact {
			p.row(id, "PASS", fmt.Sprintf("未预建的标签已保存在联系人上（读回确认）；是否出现在 AdminTags：%v", inAdmin), files...)
		} else {
			p.row(id, "UNKNOWN", "联系人创建成功，但读回时没有这个标签：未预建的标签可能被丢弃", files...)
		}
	case "F2":
		l, el := call("listings")
		u, eu := call("users")
		p.vars["listing"] = probeFirst(l)
		p.vars["user"] = probeFirst(u)
		// Shared sandbox: never assign to other integrators' users; use our own user (--remindee) when given.
		if p.vars["remindee"] != "" {
			p.vars["user"] = p.vars["remindee"]
		}
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
		_, e3 := call("propertyDnd")
		e2 = errors.Join(e2, e3)
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
	for _, secret := range []string{c.Token, c.Password} {
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
		// Rex has no generic delete: purge (leads/reminders) > trash > archive.
		for _, m := range []string{"purge", "trash", "archive"} {
			if _, _, err = c.Call(ctx, r.Service, m, map[string]any{"id": r.ID}); err == nil {
				break
			}
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
	cfg := loadRexConfig()
	if len(cfg.Missing) > 0 {
		return fmt.Errorf("Rex disabled: %s", cfg.ReadReason)
	}
	c := newRexClient(cfg.BaseURL, cfg.AppID)
	c.AccountID = cfg.AccountID
	c.Caller = "probe"
	c.Password = cfg.Password
	if ms, err := strconv.Atoi(os.Getenv("REX_INTERVAL_MS")); err == nil && ms >= 0 {
		c.Interval = time.Duration(ms) * time.Millisecond
	}
	email, password, account := cfg.Email, cfg.Password, cfg.AccountID
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
		attachProbeLog(c, filepath.Dir(*state))
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
	if *confirmed == "" || *confirmed != account {
		return errors.New("--account must match REX_ACCOUNT_ID; no Rex calls performed")
	}
	if *out == "" {
		*out = fmt.Sprintf("rex-probe-%s", time.Now().UTC().Format("20060102-150405"))
	}
	if err := os.MkdirAll(filepath.Join(*out, "raw"), 0700); err != nil {
		return err
	}
	attachProbeLog(c, *out)
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
