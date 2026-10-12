package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescribeCommandOutput(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 100000)
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"nil", nil, "no output"},
		{"empty", []byte{}, "no output"},
		{"short", []byte("abc"), "3 bytes of output omitted"},
		{"multiline", []byte("line1\nline2\n"), "12 bytes of output omitted"},
		{"very long", []byte(long), "100000 bytes of output omitted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DescribeCommandOutput(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
