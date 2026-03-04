package util

import (
	"crypto/subtle"
)

// ConstantTimeCompare compares two strings in constant time to prevent comparison timing attacks.
//
// PROTECTION SCOPE:
// This function protects against timing attacks during the comparison operation itself,
// where an attacker might measure how long it takes to compare two strings byte-by-byte
// to infer information about the expected value.
//
// IMPORTANT LIMITATIONS:
// This does NOT protect against timing variance in database queries. If you retrieve a token
// from the database (e.g., WHERE invite_token = ?), the DB query timing will vary based on
// whether the token exists, potentially revealing information to an attacker through timing analysis.
// See backend/internal/api/handlers/user_handler.go for examples of this limitation.
//
// DEFENSE-IN-DEPTH:
// Despite this limitation, using constant-time comparison is still valuable as part of a
// defense-in-depth strategy. It eliminates one potential timing leak and should be used
// when comparing sensitive values like API keys, tokens, or passwords that are already
// in memory.
//
// Returns true if the strings are equal, false otherwise.
func ConstantTimeCompare(a, b string) bool {
	aBytes := []byte(a)
	bBytes := []byte(b)

	// subtle.ConstantTimeCompare returns 1 if equal, 0 if not
	return subtle.ConstantTimeCompare(aBytes, bBytes) == 1
}

// ConstantTimeCompareBytes compares two byte slices in constant time to prevent comparison timing attacks.
//
// This function has the same protection scope and limitations as ConstantTimeCompare.
// See ConstantTimeCompare documentation for details on what this protects against and
// what it does NOT protect against (e.g., database query timing variance).
func ConstantTimeCompareBytes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
