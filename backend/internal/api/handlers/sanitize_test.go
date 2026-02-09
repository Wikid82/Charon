package handlers

import (
	"testing"
)

func TestSanitizeForLog(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"normal text", "normal text"},
		{"line\nbreak", "line break"},
		{"carriage\rreturn\nline", "carriage return line"},
		{"control\x00chars", "control chars"},
	}

	for _, tc := range cases {
		got := sanitizeForLog(tc.in)
		if got != tc.want {
			t.Fatalf("sanitizeForLog(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
