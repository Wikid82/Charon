package handlers

import (
	"regexp"
	"strings"
)

// sanitizeForLog removes control characters and newlines from user content before logging.
func sanitizeForLog(s string) string {
	if s == "" {
		return s
	}
	// Replace CRLF and LF with spaces and remove other control chars
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	// remove any other non-printable control characters
	re := regexp.MustCompile(`[\x00-\x1F\x7F]+`)
	s = re.ReplaceAllString(s, " ")
	return s
}
