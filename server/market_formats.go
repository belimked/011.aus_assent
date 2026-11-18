package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/extrame/xls"
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
		wb, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
		if err != nil {
			return nil, err
		}
		if wb.NumSheets() == 0 {
			return nil, fmt.Errorf("no sheets")
		}
		sheet := wb.GetSheet(0)
		rows := make([][]string, 0, int(sheet.MaxRow)+1)
		for i := 0; i <= int(sheet.MaxRow); i++ {
			rows = append(rows, xlsMarketRow(sheet, i))
		}
		return rows, nil
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

// extrame/xls panics on missing row records; treat those as blank worksheet rows.
func xlsMarketRow(sheet *xls.WorkSheet, index int) (out []string) {
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	row := sheet.Row(index)
	out = make([]string, row.LastCol())
	for i := range out {
		out[i] = row.Col(i)
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
