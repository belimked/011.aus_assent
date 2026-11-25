// Command gen-fake-rpdata writes SYNTHETIC RP Data (CoreLogic / Cotality)
// property-export workbooks for the test server. Addresses and lots come from
// the public QLD parcel file (web/public/parcels.json, CC BY 4.0); every owner,
// vendor, agent, agency, price and date is invented. Output must never be
// imported into production. See docs/rpdata-synthetic.md.
//
//	go run ./cmd/gen-fake-rpdata --out ../.tmp/fakerp/out
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Header copied from the CoreLogic export structure (row 3; row 1 is the
// search string, row 2 is blank). Column order matters: the importer reads the
// RP Data link from column AD (index 29).
var header = []string{"Property Photo", "Street Address", "Suburb", "State", "Postcode", "Council Area", "Property Type", "Bed", "Bath", "Car", "Land Size (m²)", "Floor Size (m²)", "Year Built", "Sale Price", "Sale Date", "Settlement Date", "Sale Type", "Agency", "Agent", "Land Use", "Development Zone", "Parcel Details", "Owner 1 Name", "Owner 2 Name", "Owner 3 Name", "Owner Type", "Vendor 1 Name", "Vendor 2 Name", "Vendor 3 Name", "Open in RPData"}

const (
	headerRow = 3
	marker    = " (Test)"
	prefix    = "SYNTHETIC_"
)

type suburb struct {
	Name     string
	Postcode string
	Real     int // approximate row count of the real export (scale reference only)
	Must     []string
}

var suburbs = []suburb{
	{"Sunnybank", "4109", 3860, []string{"Gager St", "Feltwell St", "Boorman St", "Lister St"}},
	{"Sunnybank Hills", "4109", 6650, nil},
	{"Calamvale", "4116", 7370, nil},
	{"Parkinson", "4115", 4010, nil},
}

type options struct {
	parcels       string
	out           string
	seed          uint64
	ratio         float64
	countPer      int
	today         time.Time
	incMin        int
	incMax        int
	recentInitial int
	noSale        float64
	only          map[string]bool
}

type lot struct {
	lp, unit, no, street, suburb string
	area                         float64
	num                          int
}

type record struct {
	lot
	kind, landUse, zone            string
	beds, baths, cars, floor, year int
	owners, vendors                []string
	ownerType, saleType            string
	agency, agent                  string
	price                          int64
	date                           time.Time
}

type fileInfo struct {
	Batch  string `json:"batch"`
	Suburb string `json:"suburb"`
	File   string `json:"file"`
	Rows   int    `json:"rows"`
	Sales  int    `json:"sales"`
	Bytes  int64  `json:"bytes"`
}

func main() {
	o, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	files, err := generate(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, f := range files {
		fmt.Printf("%-11s %-16s rows=%-4d sales=%-4d %7d B  %s\n", f.Batch, f.Suburb, f.Rows, f.Sales, f.Bytes, f.File)
	}
}

func parseFlags(args []string) (options, error) {
	fs := flag.NewFlagSet("gen-fake-rpdata", flag.ContinueOnError)
	var o options
	var today, only string
	fs.StringVar(&o.parcels, "parcels", "../web/public/parcels.json", "public QLD parcel file")
	fs.StringVar(&o.out, "out", "../.tmp/fakerp/out", "output directory (initial/ and incremental/ are created)")
	fs.Uint64Var(&o.seed, "seed", 20261125, "random seed; same seed and --today give identical files")
	fs.Float64Var(&o.ratio, "ratio", 0.10, "rows per suburb as a share of the real export size")
	fs.IntVar(&o.countPer, "count-per-suburb", 0, "fixed rows per suburb (overrides --ratio)")
	fs.StringVar(&today, "today", "", "reference date YYYY-MM-DD (default: today in Brisbane)")
	fs.IntVar(&o.incMin, "incremental-min", 3, "minimum new sales per suburb in the incremental batch")
	fs.IntVar(&o.incMax, "incremental-max", 5, "maximum new sales per suburb in the incremental batch")
	fs.IntVar(&o.recentInitial, "recent-initial", 0, "initial-batch sales per suburb dated within the last 90 days (these raise alerts; default none)")
	fs.Float64Var(&o.noSale, "no-sale-ratio", 0.15, "share of properties without any sale record")
	fs.StringVar(&only, "suburbs", "", "comma-separated subset of: Sunnybank,Sunnybank Hills,Calamvale,Parkinson")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	brisbane, err := time.LoadLocation("Australia/Brisbane")
	if err != nil {
		return o, err
	}
	if today == "" {
		n := time.Now().In(brisbane)
		o.today = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	} else if o.today, err = time.Parse("2006-01-02", today); err != nil {
		return o, fmt.Errorf("--today: %w", err)
	}
	if o.ratio <= 0 || o.ratio > 1 {
		return o, errors.New("--ratio must be in (0,1]")
	}
	if o.incMin < 0 || o.incMax < o.incMin {
		return o, errors.New("invalid --incremental-min/max")
	}
	if o.noSale < 0 || o.noSale >= 1 {
		return o, errors.New("--no-sale-ratio must be in [0,1)")
	}
	if only != "" {
		o.only = map[string]bool{}
		for _, s := range strings.Split(only, ",") {
			s = strings.TrimSpace(s)
			found := false
			for _, sub := range suburbs {
				if strings.EqualFold(sub.Name, s) {
					o.only[sub.Name], found = true, true
				}
			}
			if !found {
				return o, fmt.Errorf("unknown suburb %q", s)
			}
		}
	}
	return o, nil
}

// loadLots returns one candidate per lot/plan for each suburb. The importer
// matches by lot first, so two rows on one lot would update the same property.
func loadLots(path string) (map[string][]lot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var src struct {
		Addresses [][]any `json:"addresses"`
		Parcels   []struct {
			LP   string  `json:"lp"`
			Area float64 `json:"area"`
		} `json:"parcels"`
	}
	if err = json.Unmarshal(b, &src); err != nil {
		return nil, err
	}
	area := map[string]float64{}
	for _, p := range src.Parcels {
		area[p.LP] = p.Area
	}
	type group struct{ addrs []lot }
	groups := map[string]*group{}
	order := []string{}
	for _, a := range src.Addresses {
		if len(a) < 5 {
			continue
		}
		s := func(i int) string { v, _ := a[i].(string); return strings.TrimSpace(v) }
		l := lot{unit: s(0), no: s(1), street: s(2), suburb: s(3), lp: strings.ToUpper(s(4))}
		if l.lp == "" || l.no == "" || l.street == "" {
			continue
		}
		l.area = area[l.lp]
		l.num, _ = strconv.Atoi(strings.TrimRightFunc(l.no, func(r rune) bool { return r < '0' || r > '9' }))
		k := l.suburb + "|" + l.lp
		if groups[k] == nil {
			groups[k] = &group{}
			order = append(order, k)
		}
		groups[k].addrs = append(groups[k].addrs, l)
	}
	out := map[string][]lot{}
	for _, k := range order {
		g := groups[k].addrs
		sort.Slice(g, func(i, j int) bool {
			return g[i].street+"|"+pad(g[i].no)+"|"+pad(g[i].unit) < g[j].street+"|"+pad(g[j].no)+"|"+pad(g[j].unit)
		})
		pick := g[0]
		if len(g) > 1 {
			// Several addresses on one lot (dual occupancy, corner lot): the
			// whole lot is one property; quote the street address without a unit.
			pick.unit = ""
		}
		out[pick.suburb] = append(out[pick.suburb], pick)
	}
	return out, nil
}

func pad(s string) string { return fmt.Sprintf("%8s", s) }

// sampleStreets picks complete streets (shuffled, required ones first) until
// the target is reached; the last street is cut to a contiguous run.
func sampleStreets(lots []lot, target int, must []string, rng *rand.Rand) []lot {
	byStreet := map[string][]lot{}
	for _, l := range lots {
		byStreet[l.street] = append(byStreet[l.street], l)
	}
	names := make([]string, 0, len(byStreet))
	for n := range byStreet {
		names = append(names, n)
	}
	sort.Strings(names)
	rng.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
	ordered := []string{}
	seen := map[string]bool{}
	for _, m := range must {
		if byStreet[m] != nil {
			ordered = append(ordered, m)
			seen[m] = true
		}
	}
	for _, n := range names {
		if !seen[n] {
			ordered = append(ordered, n)
		}
	}
	var out []lot
	for _, n := range ordered {
		if len(out) >= target {
			break
		}
		street := byStreet[n]
		sort.Slice(street, func(i, j int) bool {
			if street[i].num != street[j].num {
				return street[i].num < street[j].num
			}
			return pad(street[i].no)+pad(street[i].unit) < pad(street[j].no)+pad(street[j].unit)
		})
		if need := target - len(out); len(street) > need {
			start := rng.IntN(len(street) - need + 1)
			street = street[start : start+need]
		}
		out = append(out, street...)
	}
	return out
}

var (
	firstNames = []string{"Avery", "Blake", "Casey", "Dana", "Ellis", "Finley", "Gray", "Harper", "Indigo", "Jordan", "Kendall", "Logan", "Morgan", "Noel", "Oakley", "Parker", "Quinn", "Reese", "Sage", "Taylor", "Umber", "Vale", "Wren", "Yael", "Zephyr"}
	lastNames  = []string{"Placeholder", "Testerton", "Sampleford", "Mockridge", "Fauxley", "Dummett", "Fictionby", "Pretendall", "Exampleton", "Synthwood", "Notreal", "Specimen", "Trialby", "Stubbings", "Fixturely", "Demoworth", "Imaginary", "Madeupson", "Quizzard", "Lorem"}
	agencies   = []string{"Northwind Realty", "Bluegum Property Group", "Contoso Estate Agents", "Fabrikam Homes", "Tailspin Real Estate", "Wingtip Property"}
	saleTypes  = []string{"Normal Sale", "Normal Sale", "Normal Sale", "Normal Sale", "Auction", "Auction", "Private Treaty", "Mortgagee Sale"}
	zones      = []string{"Low Density Residential", "Low Density Residential", "Low-medium Density Residential", "Medium Density Residential"}
)

func person(rng *rand.Rand) string {
	return firstNames[rng.IntN(len(firstNames))] + " " + lastNames[rng.IntN(len(lastNames))] + marker
}

func people(rng *rand.Rand) []string {
	n := 1
	if r := rng.Float64(); r < 0.08 {
		n = 3
	} else if r < 0.55 {
		n = 2
	}
	out := []string{person(rng)}
	for len(out) < n {
		// Co-owners usually share a surname.
		last := strings.Fields(out[0])[1]
		out = append(out, firstNames[rng.IntN(len(firstNames))]+" "+last+marker)
	}
	return out
}

// value returns an invented current market value; older sales are discounted
// by ~5% a year so a 2011 sale reads plausibly lower than a 2026 one.
func value(r record, rng *rand.Rand, today, date time.Time) int64 {
	var v float64
	switch r.kind {
	case "Unit":
		v = 380000 + rng.Float64()*370000
	case "Townhouse":
		v = 560000 + rng.Float64()*340000
	case "Land":
		v = 650000 + math.Min(r.area, 1500)*500 + rng.Float64()*150000
	default:
		v = 800000 + math.Min(math.Max(r.area-400, 0), 800)*1000 + float64(r.beds-3)*60000 + rng.Float64()*350000
		v = math.Min(math.Max(v, 800000), 2000000)
	}
	years := today.Sub(date).Hours() / 24 / 365.25
	v /= math.Pow(1.05, years)
	return int64(math.Round(v/5000) * 5000)
}

func describe(l lot, rng *rand.Rand) record {
	r := record{lot: l}
	switch {
	case l.unit != "" && (l.area >= 150 || rng.Float64() < 0.35):
		r.kind, r.landUse = "Townhouse", "Building Units"
		r.beds, r.baths, r.cars = 3+rng.IntN(2), 2, 1+rng.IntN(2)
		r.floor, r.year = 110+rng.IntN(80), 1995+rng.IntN(30)
	case l.unit != "":
		r.kind, r.landUse = "Unit", "Building Units"
		r.beds, r.baths, r.cars = 1+rng.IntN(3), 1+rng.IntN(2), 1
		r.floor, r.year = 55+rng.IntN(70), 1985+rng.IntN(40)
	case rng.Float64() < 0.03:
		r.kind, r.landUse = "Land", "Vacant Urban Land"
	default:
		r.kind, r.landUse = "House", "Single Unit Dwelling"
		r.beds = 3 + rng.IntN(3)
		r.baths = 1 + rng.IntN(3)
		r.cars = 1 + rng.IntN(3)
		r.floor = 120 + r.beds*25 + rng.IntN(120)
		r.year = 1960 + rng.IntN(63)
	}
	r.zone = zones[rng.IntN(len(zones))]
	if r.kind == "Unit" || r.kind == "Townhouse" {
		r.zone = "Medium Density Residential"
	}
	r.owners = people(rng)
	r.ownerType = "Owner Occupied"
	if rng.Float64() < 0.3 {
		r.ownerType = "Rented"
	}
	return r
}

func addSale(r *record, rng *rand.Rand, today, date time.Time) {
	r.date = date
	r.price = value(*r, rng, today, date)
	r.saleType = saleTypes[rng.IntN(len(saleTypes))]
	r.agency = agencies[rng.IntN(len(agencies))] + marker
	r.agent = person(rng)
	r.vendors = people(rng)
}

func generate(o options) ([]fileInfo, error) {
	all, err := loadLots(o.parcels)
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{"initial", "incremental"} {
		if err := os.MkdirAll(filepath.Join(o.out, dir), 0o755); err != nil {
			return nil, err
		}
	}
	var files []fileInfo
	for si, sub := range suburbs {
		if o.only != nil && !o.only[sub.Name] {
			continue
		}
		rng := rand.New(rand.NewPCG(o.seed, uint64(si+1)))
		target := o.countPer
		if target <= 0 {
			target = int(math.Round(float64(sub.Real) * o.ratio))
		}
		lots := sampleStreets(all[sub.Name], target, sub.Must, rng)
		if len(lots) == 0 {
			return nil, fmt.Errorf("no parcels for %s in %s", sub.Name, o.parcels)
		}
		recs := make([]record, len(lots))
		oldest := o.today.AddDate(-15, 0, 0)
		latest := o.today.AddDate(0, 0, -91) // outside the importer's 90-day alert window
		span := int(latest.Sub(oldest).Hours() / 24)
		recent := map[int]bool{}
		for _, i := range rng.Perm(len(lots))[:min(o.recentInitial, len(lots))] {
			recent[i] = true
		}
		for i, l := range lots {
			recs[i] = describe(l, rng)
			if recent[i] {
				addSale(&recs[i], rng, o.today, o.today.AddDate(0, 0, -(8+rng.IntN(80))))
			} else if rng.Float64() >= o.noSale {
				addSale(&recs[i], rng, o.today, oldest.AddDate(0, 0, rng.IntN(span+1)))
			}
		}
		clock := func(day time.Time) time.Time {
			return day.Add(time.Duration(9*3600+rng.IntN(8*3600)) * time.Second)
		}
		initialAt := clock(o.today.AddDate(0, 0, -8))
		f, err := write(filepath.Join(o.out, "initial"), sub, recs, initialAt)
		if err != nil {
			return nil, err
		}
		f.Batch = "initial"
		files = append(files, f)

		// Incremental batch: a few of the same properties sold again within the last 7 days.
		n := o.incMin
		if o.incMax > o.incMin {
			n += rng.IntN(o.incMax - o.incMin + 1)
		}
		var inc []record
		for _, i := range rng.Perm(len(recs))[:min(n, len(recs))] {
			r := recs[i]
			previous := r.owners
			date := o.today.AddDate(0, 0, -rng.IntN(7))
			prior := r.price
			addSale(&r, rng, o.today, date)
			if r.price <= prior {
				r.price = int64(math.Round(float64(prior)*(1.04+rng.Float64()*0.08)/5000) * 5000)
			}
			r.vendors = previous
			r.owners = people(rng)
			r.ownerType = "Owner Occupied"
			inc = append(inc, r)
		}
		f, err = write(filepath.Join(o.out, "incremental"), sub, inc, clock(o.today))
		if err != nil {
			return nil, err
		}
		f.Batch = "incremental"
		files = append(files, f)
	}
	return files, nil
}

var lotPlan = regexp.MustCompile(`^([0-9]+)([A-Z]+[0-9]+)$`)

func parcelDetails(lp string) string {
	if m := lotPlan.FindStringSubmatch(lp); m != nil {
		return "Lot " + m[1] + " " + m[2]
	}
	return lp
}

func dash(v int) any {
	if v <= 0 {
		return "-"
	}
	return v
}

func write(dir string, sub suburb, recs []record, at time.Time) (fileInfo, error) {
	info := fileInfo{Suburb: sub.Name, Rows: len(recs)}
	sort.SliceStable(recs, func(i, j int) bool {
		a, b := recs[i], recs[j]
		if a.street != b.street {
			return a.street < b.street
		}
		if a.num != b.num {
			return a.num < b.num
		}
		return pad(a.no)+pad(a.unit) < pad(b.no)+pad(b.unit)
	})
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	stamp := at.Format("2006-01-02T15:04:05Z")
	if err := f.SetDocProps(&excelize.DocProperties{Creator: "gen-fake-rpdata (SYNTHETIC)", Title: "SYNTHETIC RP Data export - invented test data, not for production", Created: stamp, Modified: stamp}); err != nil {
		return info, err
	}
	head, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Family: "Arial", Size: 11, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"808080"}}})
	if err != nil {
		return info, err
	}
	postcode := "#0000"
	postStyle, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &postcode})
	size := "#,###,###,##0"
	sizeStyle, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &size})
	last, _ := excelize.ColumnNumberToName(len(header))
	_ = f.SetColWidth(sheet, "A", last, 20)
	_ = f.SetSheetRow(sheet, "A1", &[]any{"Search String", sub.Name + " QLD " + sub.Postcode})
	_ = f.SetSheetRow(sheet, "A3", &header)
	_ = f.SetCellStyle(sheet, "A3", last+"3", head)
	pc, _ := strconv.Atoi(sub.Postcode)
	for i, r := range recs {
		row := headerRow + 1 + i
		addr := r.no + " " + r.street
		if r.unit != "" {
			addr = r.unit + "/" + addr
		}
		land := any("-")
		if r.unit == "" && r.area > 0 {
			land = int(math.Round(r.area))
		}
		cells := []any{"", addr, r.suburb, "QLD", pc, "Brisbane", r.kind, dash(r.beds), dash(r.baths), dash(r.cars), land, dash(r.floor), dash(r.year)}
		if r.date.IsZero() {
			cells = append(cells, "-", "-", "-", "-", "-", "-")
		} else {
			settle := "-"
			if s := r.date.AddDate(0, 0, 30+int(r.price/5000)%31); r.date.Before(at.AddDate(0, 0, -60)) {
				settle = s.Format("2 Jan 2006")
			}
			cells = append(cells, "$"+commas(r.price), r.date.Format("2 Jan 2006"), settle, r.saleType, r.agency, r.agent)
		}
		cells = append(cells, r.landUse, r.zone, parcelDetails(r.lp))
		for k := 0; k < 3; k++ {
			cells = append(cells, nth(r.owners, k))
		}
		cells = append(cells, r.ownerType)
		for k := 0; k < 3; k++ {
			cells = append(cells, nth(r.vendors, k))
		}
		cells = append(cells, "") // Open in RPData: no links in synthetic files
		cell, _ := excelize.CoordinatesToCellName(1, row)
		if err := f.SetSheetRow(sheet, cell, &cells); err != nil {
			return info, err
		}
		_ = f.SetCellStyle(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), postStyle)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("K%d", row), fmt.Sprintf("L%d", row), sizeStyle)
		if !r.date.IsZero() {
			info.Sales++
		}
	}
	name := prefix + sub.Name + "_RPData_propertyExport_" + at.Format("20060102150405") + ".xlsx"
	info.File = filepath.Join(dir, name)
	if err := f.SaveAs(info.File); err != nil {
		return info, err
	}
	st, err := os.Stat(info.File)
	if err != nil {
		return info, err
	}
	info.Bytes = st.Size()
	return info, nil
}

func nth(v []string, i int) string {
	if i < len(v) {
		return v[i]
	}
	return "-"
}

func commas(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
