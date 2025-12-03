package util

import (
	"regexp"
	"strings"
)

// SanitizeForLog removes control characters and newlines from user content before logging.
func SanitizeForLog(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	re := regexp.MustCompile(`[\x00-\x1F\x7F]+`)
	s = re.ReplaceAllString(s, " ")
	return s
}
