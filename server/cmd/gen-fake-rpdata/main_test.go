package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func fixtureParcels(t *testing.T) string {
	t.Helper()
	var addrs [][]any
	var parcels []map[string]any
	lot := 1
	add := func(unit, no, street, sub string) {
		lp := fmt.Sprintf("%dRP%d", lot, 800000+len(sub))
		lot++
		addrs = append(addrs, []any{unit, no, street, sub, lp, 0, 0, lot})
		parcels = append(parcels, map[string]any{"lp": lp, "area": 400 + lot, "r": [][]int{}})
	}
	for _, sub := range []string{"Sunnybank", "Sunnybank Hills", "Calamvale", "Parkinson", "Runcorn"} {
		for s, street := range []string{"Gager St", "Feltwell St", "Boorman St", "Lister St", "Example Rd", "Sample Ct", "Fixture Ave"} {
			for n := 1; n <= 6+s; n++ {
				add("", fmt.Sprint(n*2), street, sub)
			}
		}
		for u := 1; u <= 4; u++ {
			add(fmt.Sprint(u), "9", "Strata Pl", sub)
		}
	}
	// Two addresses on one lot (corner lot) must give a single row.
	addrs = append(addrs, []any{"", "1", "Corner St", "Sunnybank", "999RP1", 0, 0, 1}, []any{"", "3", "Other St", "Sunnybank", "999RP1", 0, 0, 2})
	b, _ := json.Marshal(map[string]any{"addresses": addrs, "parcels": parcels})
	path := filepath.Join(t.TempDir(), "parcels.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func run(t *testing.T, parcels string, extra ...string) (string, []fileInfo) {
	t.Helper()
	out := t.TempDir()
	o, err := parseFlags(append([]string{"--parcels", parcels, "--out", out, "--today", "2026-11-25", "--count-per-suburb", "30"}, extra...))
	if err != nil {
		t.Fatal(err)
	}
	files, err := generate(o)
	if err != nil {
		t.Fatal(err)
	}
	return out, files
}

func TestGenerateStructureAndPrivacy(t *testing.T) {
	parcels := fixtureParcels(t)
	_, files := run(t, parcels)
	if len(files) != 8 {
		t.Fatal("files", len(files))
	}
	today, _ := time.Parse("2006-01-02", "2026-11-25")
	for _, fi := range files {
		base := filepath.Base(fi.File)
		if !strings.HasPrefix(base, "SYNTHETIC_"+fi.Suburb+"_RPData_propertyExport_") || !strings.HasSuffix(base, ".xlsx") {
			t.Fatal("name", base)
		}
		f, err := excelize.OpenFile(fi.File)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.GetSheetList(); len(got) != 1 || got[0] != "Sheet1" {
			t.Fatal("sheets", got)
		}
		rows, _ := f.GetRows("Sheet1")
		if rows[0][0] != "Search String" || !strings.HasPrefix(rows[0][1], fi.Suburb+" QLD 41") || len(rows[1]) != 0 || strings.Join(rows[2], "|") != strings.Join(header, "|") {
			t.Fatal("preamble/header", rows[:3])
		}
		data := rows[headerRow:]
		if len(data) != fi.Rows {
			t.Fatal("rows", len(data), fi.Rows)
		}
		if fi.Batch == "initial" && fi.Rows != 30 || fi.Batch == "incremental" && (fi.Rows < 3 || fi.Rows > 5) {
			t.Fatal("count", fi.Batch, fi.Rows)
		}
		lots := map[string]bool{}
		streets := map[string]bool{}
		for i, r := range data {
			col := func(name string) string {
				for j, h := range header {
					if h == name && j < len(r) {
						return r[j]
					}
				}
				return ""
			}
			if col("Suburb") != fi.Suburb || !strings.HasSuffix(col("Owner 1 Name"), " (Test)") || lots[col("Parcel Details")] {
				t.Fatal("row", i, r)
			}
			for _, name := range []string{"Owner 2 Name", "Owner 3 Name", "Vendor 1 Name", "Vendor 2 Name", "Vendor 3 Name", "Agent", "Agency"} {
				if v := col(name); v != "-" && !strings.HasSuffix(v, " (Test)") {
					t.Fatal("not marked as test", name, v)
				}
			}
			lots[col("Parcel Details")] = true
			addr := col("Street Address")
			streets[addr[strings.Index(addr, " ")+1:]] = true
			if col("Sale Date") != "-" {
				d, err := time.Parse("2 Jan 2006", col("Sale Date"))
				if err != nil || !strings.HasPrefix(col("Sale Price"), "$") {
					t.Fatal("sale", r)
				}
				age := today.Sub(d).Hours() / 24
				if fi.Batch == "initial" && (age <= 90 || age > 366*15) || fi.Batch == "incremental" && (age < 0 || age > 6) {
					t.Fatal("sale date outside batch window", fi.Batch, d)
				}
			} else if fi.Batch == "incremental" {
				t.Fatal("incremental row without sale")
			}
			cell, _ := excelize.CoordinatesToCellName(len(header), headerRow+1+i)
			if formula, _ := f.GetCellFormula("Sheet1", cell); formula != "" {
				t.Fatal("formula link")
			}
			if ok, _, _ := f.GetCellHyperLink("Sheet1", cell); ok {
				t.Fatal("hyperlink")
			}
		}
		if fi.Suburb == "Sunnybank" && fi.Batch == "initial" {
			for _, s := range []string{"Gager St", "Feltwell St", "Boorman St", "Lister St"} {
				if !streets[s] {
					t.Fatal("required street missing", s, streets)
				}
			}
		}
		if fi.Suburb == "Sunnybank" && fi.Batch == "initial" && lots["Lot 999 RP1"] && (streets["Corner St"] == streets["Other St"]) {
			t.Fatal("corner lot exported twice")
		}
		f.Close()
	}
}

func TestGenerateReproducible(t *testing.T) {
	parcels := fixtureParcels(t)
	digest := func(files []fileInfo) string {
		h := sha256.New()
		for _, f := range files {
			b, err := os.ReadFile(f.File)
			if err != nil {
				t.Fatal(err)
			}
			h.Write([]byte(filepath.Base(f.File)))
			h.Write(b)
		}
		return fmt.Sprintf("%x", h.Sum(nil))
	}
	_, a := run(t, parcels)
	_, b := run(t, parcels)
	_, c := run(t, parcels, "--seed", "7")
	if digest(a) != digest(b) {
		t.Fatal("same seed differs")
	}
	if digest(a) == digest(c) {
		t.Fatal("seed ignored")
	}
}

func TestRecentInitialAndRatio(t *testing.T) {
	parcels := fixtureParcels(t)
	out := t.TempDir()
	o, err := parseFlags([]string{"--parcels", parcels, "--out", out, "--today", "2026-11-25", "--ratio", "0.005", "--suburbs", "Parkinson", "--recent-initial", "2", "--no-sale-ratio", "0"})
	if err != nil {
		t.Fatal(err)
	}
	files, err := generate(o)
	if err != nil || len(files) != 2 || files[0].Rows != 20 { // 4010 * 0.005
		t.Fatal(files, err)
	}
	f, _ := excelize.OpenFile(files[0].File)
	defer f.Close()
	rows, _ := f.GetRows("Sheet1")
	recent := 0
	today, _ := time.Parse("2006-01-02", "2026-11-25")
	for _, r := range rows[headerRow:] {
		if d, err := time.Parse("2 Jan 2006", r[14]); err == nil && today.Sub(d).Hours()/24 <= 90 {
			recent++
		}
	}
	if recent != 2 {
		t.Fatal("recent initial sales", recent)
	}
	if _, err := parseFlags([]string{"--suburbs", "Runcorn"}); err == nil {
		t.Fatal("unknown suburb accepted")
	}
}
