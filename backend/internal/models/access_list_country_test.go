package models

import "testing"

func TestNormalizeCountryCodes(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"us":        "US",
		" us , ca ": "US,CA",
		"Us,cA,de":  "US,CA,DE",
		"US,,CA,":   "US,CA",
		" , ":       "",
		"GB":        "GB",
	}
	for raw, want := range cases {
		if got := NormalizeCountryCodes(raw); got != want {
			t.Errorf("NormalizeCountryCodes(%q) = %q, want %q", raw, got, want)
		}
	}
}
