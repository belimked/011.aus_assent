package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shakinm/xlsReader/xls"
	"github.com/xuri/excelize/v2"
)

func normalMarketExcelDate(value string, data []byte) string {
	if !bytes.HasPrefix(data, []byte{0xd0, 0xcf, 0x11, 0xe0}) && !bytes.HasPrefix(data, []byte{'P', 'K', 3, 4}) {
		return value
	}
	serial, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || serial <= 0 {
		return value
	}
	displayed, err := excelize.ExcelDateToTime(serial, false)
	if err != nil {
		return value
	}
	return displayed.Format("2006-01-02")
}

func marketRows(data []byte) ([][]string, error) {
	if bytes.HasPrefix(data, []byte{0xd0, 0xcf, 0x11, 0xe0}) {
		return xlsRows(data)
	}
	if bytes.HasPrefix(data, []byte{'P', 'K', 3, 4}) {
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, fmt.Errorf("no sheets")
		}
		return f.GetRows(sheets[0])
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

// xlsRows reads the first sheet of a legacy .xls (BIFF8) workbook.
// extrame/xls v0.0.1 mis-indexes the shared string table on large Pricefinder
// exports (owner names land in other columns), and its master branch formats
// every formatted number as a date; shakinm/xlsReader reads both correctly.
// Numbers and dates come back raw (Excel serials), as normalMarketExcelDate expects.
func xlsRows(data []byte) (rows [][]string, err error) {
	defer func() {
		if r := recover(); r != nil {
			rows, err = nil, fmt.Errorf("unreadable xls: %v", r)
		}
	}()
	wb, err := xls.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if wb.GetNumberSheets() == 0 {
		return nil, fmt.Errorf("no sheets")
	}
	sheet, err := wb.GetSheet(0)
	if err != nil {
		return nil, err
	}
	n := sheet.GetNumberRows()
	rows = make([][]string, 0, n)
	width := 0
	for i := 0; i < n; i++ {
		row := xlsRow(sheet, i, width)
		if i == 0 {
			width = len(row)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// xlsRow returns one row padded to the header width; missing rows are blank.
func xlsRow(sheet *xls.Sheet, index, width int) (out []string) {
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	r, err := sheet.GetRow(index)
	if err != nil || r == nil {
		return nil
	}
	n := width
	if n == 0 {
		n = len(r.GetCols())
	}
	out = make([]string, n)
	for i := range out {
		if c, e := r.GetCol(i); e == nil && c != nil {
			out[i] = c.GetString()
		}
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}
func marketDate(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02", "2/1/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("invalid date")
}
func hasColumn(cols map[string]int, name string) bool { _, ok := cols[name]; return ok }
func footerOnlyMarketRow(row []string) bool {
	nonempty := 0
	for _, v := range row {
		if strings.TrimSpace(v) != "" {
			nonempty++
		}
	}
	return nonempty <= 1
}
func ignoredMarketRow(row []string) bool {
	first := ""
	for _, s := range row {
		if strings.TrimSpace(s) != "" {
			first = strings.ToLower(strings.TrimSpace(s))
			break
		}
	}
	if first == "" {
		return true
	}
	if footerOnlyMarketRow(row) && (strings.Contains(first, "copyright") || strings.Contains(first, "disclaimer") || strings.Contains(first, "pricefinder")) {
		return true
	}
	return strings.Contains(first, "copyright") || strings.Contains(first, "disclaimer") || strings.Contains(first, "all rights reserved") || strings.Contains(first, "pricefinder") && len(first) > 60 || len(first) > 80
}

var marketRawColumns = map[string]bool{
	"address": true, "street": true, "streetaddress": true, "propertyaddress": true, "locality": true, "suburb": true, "postcode": true,
	"legaldescription": true, "lotplan": true, "area": true, "land": true, "landsize": true, "beds": true, "bedrooms": true, "baths": true, "bathrooms": true, "cars": true, "car": true, "carparks": true, "parking": true,
	"landuse": true, "zoning": true, "valuationdate": true, "valuationamount": true, "lastsale": true, "lastsaledate": true, "lastsaletype": true, "saleprice": true, "saledate": true, "price": true, "eventdate": true, "listeddate": true, "pdsid": true, "providerid": true, "recordid": true, "propertyid": true, "listingid": true, "saleid": true, "loaddate": true, "buildingname": true,
	"kind": true, "eventtype": true, "type": true,
}

func safeMarketHeader(h string) bool { return marketRawColumns[key(h)] }
func getOwnerAddress(head, line []string) string {
	for i, h := range head {
		if key(h) == "currentownersaddress" && i < len(line) {
			return strings.TrimSpace(line[i])
		}
	}
	return ""
}
func absenteeMarket(ownerAddress, address string) bool {
	if ownerAddress == "" || strings.EqualFold(ownerAddress, "N/A") {
		return false
	}
	street := strings.Split(ownerAddress, ",")[0]
	return addressKey(street) != addressKey(address)
}
func recentMarketSale(date string) bool {
	brisbane, _ := time.LoadLocation("Australia/Brisbane")
	today := time.Now().In(brisbane)
	d, err := time.ParseInLocation("2006-01-02", date, brisbane)
	if err != nil {
		return false
	}
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, brisbane)
	return !d.After(day) && !d.Before(day.AddDate(0, 0, -90))
}
