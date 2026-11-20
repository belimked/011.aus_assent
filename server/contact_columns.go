package main

// contactColumns is the single projection of contact JSON into the indexed and
// privacy-sensitive columns. Keep every contact data write paired with this projection.
func contactColumns(m map[string]any) []any {
	return []any{jsonField(m, "roles"), jsonField(m, "tags"), jsonField(m, "flags"), jsonField(m, "consent")}
}
