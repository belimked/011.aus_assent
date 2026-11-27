package main

import (
	"database/sql"
	"os"
	"strings"
)

// competitorListingFrom derives "currently listed with another agency" for a
// property from imported market data (UAT3: competitor marker). It sets
// m["competitorListing"] = {agency, date, provider} or removes the key.
//
// Source: market_events rows linked to the property. The latest dated event of
// kind listing / sale / withdrawn decides; it counts only when it is a listing
// with an agency name, strictly newer than every sale or withdrawal event and
// than property_ownership.last_sale_date (ties go to the sale / withdrawal).
// Listing events from provider "rpdata" are ignored: the built-in RP Data
// mapping reads "Agency" and "Sale Date", the agency and date of the last sale,
// not a current listing. The agency equal to this customer's company name
// (company_name setting / AXLEO_COMPANY_NAME, compared ignoring case, spaces
// and punctuation) is not a competitor.
//
// No expiry is applied: a listing stays "current" until a later sale or
// withdrawal event is imported. The phone gives this agency's own Rex listing
// (status listed) precedence over this field.
func competitorListingFrom(db querier, m map[string]any) {
	delete(m, "competitorListing")
	id := str(m, "id")
	if id == "" {
		return
	}
	var kind, agency, date string
	var provider string
	err := db.QueryRow(`SELECT kind,COALESCE(agency,''),event_date,provider FROM market_events
		WHERE property_id=? AND kind IN ('listing','sale','withdrawn') AND COALESCE(event_date,'')!=''
		AND NOT (kind='listing' AND provider='rpdata')
		ORDER BY event_date DESC, CASE kind WHEN 'listing' THEN 1 ELSE 0 END, id DESC LIMIT 1`, id).Scan(&kind, &agency, &date, &provider)
	if err != nil || kind != "listing" || strings.TrimSpace(agency) == "" {
		return
	}
	var sold sql.NullString
	_ = db.QueryRow(`SELECT last_sale_date FROM property_ownership WHERE property_id=?`, id).Scan(&sold)
	if sold.String != "" && sold.String >= date {
		return
	}
	var company string
	_ = db.QueryRow(`SELECT value FROM settings WHERE key='company_name'`).Scan(&company)
	if strings.TrimSpace(company) == "" {
		company = os.Getenv("AXLEO_COMPANY_NAME")
	}
	if c := key(company); c != "" && c == key(agency) {
		return
	}
	m["competitorListing"] = map[string]any{"agency": strings.TrimSpace(agency), "date": date, "provider": provider}
}
