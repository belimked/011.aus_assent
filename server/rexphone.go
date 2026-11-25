package main

import "strings"

func normAUMobile(value string) string {
	var b strings.Builder
	for _, c := range value {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	digits := b.String()
	if strings.HasPrefix(digits, "0061") {
		digits = digits[4:]
	} else if strings.HasPrefix(digits, "61") {
		digits = digits[2:]
	} else {
		return digits
	}
	if strings.HasPrefix(digits, "0") {
		return digits
	}
	return "0" + digits
}
