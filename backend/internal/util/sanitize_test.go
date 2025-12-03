package util

import "testing"

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "clean string",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "string with newline",
			input:    "Hello\nWorld",
			expected: "Hello World",
		},
		{
			name:     "string with carriage return and newline",
			input:    "Hello\r\nWorld",
			expected: "Hello World",
		},
		{
			name:     "string with multiple newlines",
			input:    "Hello\nWorld\nTest",
			expected: "Hello World Test",
		},
		{
			name:     "string with control characters",
			input:    "Hello\x00\x01\x1FWorld",
			expected: "Hello World",
		},
		{
			name:     "string with DEL character (0x7F)",
			input:    "Hello\x7FWorld",
			expected: "Hello World",
		},
		{
			name:     "complex string with mixed control chars",
			input:    "Line1\r\nLine2\nLine3\x00\x01\x7F",
			expected: "Line1 Line2 Line3 ",
		},
		{
			name:     "string with tabs (0x09 is control char)",
			input:    "Hello\tWorld",
			expected: "Hello World",
		},
		{
			name:     "string with only control chars",
			input:    "\x00\x01\x02\x1F\x7F",
			expected: " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLog(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeForLog(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
