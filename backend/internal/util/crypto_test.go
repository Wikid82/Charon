package util

import (
	"testing"
)

func TestConstantTimeCompare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{"equal strings", "secret123", "secret123", true},
		{"different strings", "secret123", "secret456", false},
		{"different lengths", "short", "muchlonger", false},
		{"empty strings", "", "", true},
		{"one empty", "notempty", "", false},
		{"unicode equal", "héllo", "héllo", true},
		{"unicode different", "héllo", "hëllo", false},
		{"special chars equal", "!@#$%^&*()", "!@#$%^&*()", true},
		{"whitespace matters", "hello ", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConstantTimeCompare(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ConstantTimeCompare(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestConstantTimeCompareBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{"equal bytes", []byte{1, 2, 3}, []byte{1, 2, 3}, true},
		{"different bytes", []byte{1, 2, 3}, []byte{1, 2, 4}, false},
		{"different lengths", []byte{1, 2}, []byte{1, 2, 3}, false},
		{"empty slices", []byte{}, []byte{}, true},
		{"nil slices", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConstantTimeCompareBytes(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ConstantTimeCompareBytes(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// BenchmarkConstantTimeCompare ensures the function remains constant-time.
func BenchmarkConstantTimeCompare(b *testing.B) {
	// #nosec G101 -- Test fixture for benchmarking constant-time comparison, not a real credential
	secret := "a]3kL9#mP2$vN7@qR5*wX1&yT4^uI8%oE0!"

	b.Run("equal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ConstantTimeCompare(secret, secret)
		}
	})

	b.Run("different_first_char", func(b *testing.B) {
		different := "b]3kL9#mP2$vN7@qR5*wX1&yT4^uI8%oE0!"
		for i := 0; i < b.N; i++ {
			ConstantTimeCompare(secret, different)
		}
	})

	b.Run("different_last_char", func(b *testing.B) {
		different := "a]3kL9#mP2$vN7@qR5*wX1&yT4^uI8%oE0?"
		for i := 0; i < b.N; i++ {
			ConstantTimeCompare(secret, different)
		}
	})
}
