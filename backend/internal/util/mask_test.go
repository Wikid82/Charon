package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", "[empty]"},
		{"short", "short123", "[REDACTED]"},
		{"15 chars", strings.Repeat("a", 15), "[REDACTED]"},
		{"16 chars", "1234567890123456", "[REDACTED]"},
		{"23 chars", "12345678901234567890123", "[REDACTED]"},
		{"24 chars", "123456789012345678901234", "1234...1234"},
		{"long", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcd...WXYZ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, MaskAPIKey(tt.input))
		})
	}
}
