package middleware

import (
	"net/http"
	"strings"

	"github.com/Wikid82/charon/backend/internal/util"
)

// SanitizeHeaders returns a map of header keys to redacted/sanitized values
// for safe logging. Sensitive headers are redacted; other values are
// sanitized using util.SanitizeForLog and truncated.
func SanitizeHeaders(h http.Header) map[string][]string {
	if h == nil {
		return nil
	}
	sensitive := map[string]struct{}{
		"authorization":       {},
		"cookie":              {},
		"set-cookie":          {},
		"proxy-authorization": {},
		"x-api-key":           {},
		"x-api-token":         {},
		"x-access-token":      {},
		"x-auth-token":        {},
		"x-api-secret":        {},
		"x-forwarded-for":     {},
	}
	out := make(map[string][]string, len(h))
	for k, vals := range h {
		keyLower := strings.ToLower(k)
		if _, ok := sensitive[keyLower]; ok {
			out[k] = []string{"<redacted>"}
			continue
		}
		sanitizedVals := make([]string, 0, len(vals))
		for _, v := range vals {
			v2 := util.SanitizeForLog(v)
			if len(v2) > 200 {
				v2 = v2[:200]
			}
			sanitizedVals = append(sanitizedVals, v2)
		}
		out[k] = sanitizedVals
	}
	return out
}

// SanitizePath prepares a request path for safe logging by removing
// control characters and truncating long values. It does not include
// query parameters.
func SanitizePath(p string) string {
	// remove query string
	if i := strings.Index(p, "?"); i != -1 {
		p = p[:i]
	}
	p = util.SanitizeForLog(p)
	if len(p) > 200 {
		p = p[:200]
	}
	return p
}
