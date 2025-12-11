package handlers

import (
	"path/filepath"
	"testing"
)

func TestIsSafePathUnderBase(t *testing.T) {
	base := filepath.FromSlash("/tmp/session")
	cases := []struct {
		name string
		want bool
	}{
		{"Caddyfile", true},
		{"site/site.conf", true},
		{"../etc/passwd", false},
		{"../../escape", false},
		{"/absolute/path", false},
		{"", false},
		{".", false},
		{"sub/../ok.txt", true},
	}

	for _, tc := range cases {
		got := isSafePathUnderBase(base, tc.name)
		if got != tc.want {
			t.Fatalf("isSafePathUnderBase(%q, %q) = %v; want %v", base, tc.name, got, tc.want)
		}
	}
}
