package util

import (
	"crypto/subtle"
)

// ConstantTimeCompare compares two strings in constant time to prevent timing attacks.
// Returns true if the strings are equal, false otherwise.
// This should be used when comparing sensitive values like tokens.
func ConstantTimeCompare(a, b string) bool {
	aBytes := []byte(a)
	bBytes := []byte(b)

	// subtle.ConstantTimeCompare returns 1 if equal, 0 if not
	return subtle.ConstantTimeCompare(aBytes, bBytes) == 1
}

// ConstantTimeCompareBytes compares two byte slices in constant time.
func ConstantTimeCompareBytes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
