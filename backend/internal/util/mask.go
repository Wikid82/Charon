package util

import "fmt"

// maskMinLength is the shortest value that may show its first and last characters.
const maskMinLength = 24

// MaskAPIKey returns the masked form of an API key for UI previews and API display
// fields only: "[empty]" for an empty value, "[REDACTED]" below the minimum length,
// otherwise the first and last four characters.
func MaskAPIKey(key string) string {
	if key == "" {
		return "[empty]"
	}
	if len(key) < maskMinLength {
		return "[REDACTED]"
	}
	return fmt.Sprintf("%s...%s", key[:4], key[len(key)-4:])
}
