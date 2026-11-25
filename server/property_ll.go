package main

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// Phones only map properties through data.ll ([lat,lng]). Some writers (the
// bundled DCDB parcels, Rex rows, older imports) carry lat/lng instead, so every
// property write normalises to ll. Bounds match the Rex import (mainland + Tas).
func plausibleAULL(lat, lng float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lng) && lat >= -44 && lat <= -9 && lng >= 112 && lng <= 154
}

func coordNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

// propertyLL returns the map point of a property: a valid ll first, then lat/lng.
func propertyLL(d map[string]any) (float64, float64, bool) {
	switch ll := d["ll"].(type) {
	case []any:
		if len(ll) == 2 {
			lat, ok1 := coordNumber(ll[0])
			lng, ok2 := coordNumber(ll[1])
			if ok1 && ok2 && plausibleAULL(lat, lng) {
				return lat, lng, true
			}
		}
	case []float64:
		if len(ll) == 2 && plausibleAULL(ll[0], ll[1]) {
			return ll[0], ll[1], true
		}
	}
	lat, ok1 := coordNumber(d["lat"])
	lng, ok2 := coordNumber(d["lng"])
	if ok1 && ok2 && plausibleAULL(lat, lng) {
		return lat, lng, true
	}
	return 0, 0, false
}

// ensurePropertyLL sets ll from lat/lng when ll is absent or unusable. It never
// overrides a usable ll and reports whether data changed.
func ensurePropertyLL(d map[string]any) bool {
	if d == nil {
		return false
	}
	lat, lng, ok := propertyLL(d)
	if !ok {
		return false
	}
	switch ll := d["ll"].(type) {
	case []any:
		if len(ll) == 2 {
			a, _ := coordNumber(ll[0])
			b, _ := coordNumber(ll[1])
			if a == lat && b == lng {
				return false
			}
		}
	case []float64:
		if len(ll) == 2 && ll[0] == lat && ll[1] == lng {
			return false
		}
	}
	d["ll"] = []float64{lat, lng}
	return true
}
