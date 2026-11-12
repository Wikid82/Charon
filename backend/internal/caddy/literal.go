package caddy

import "strings"

// placeholderDelimiterReplacer percent-encodes the characters Caddy uses to
// open and close placeholders. Percent-encoding is valid in URLs, host
// matchers, path matchers and dial addresses, so an encoded value reads the
// same to the client while Caddy no longer expands it.
var placeholderDelimiterReplacer = strings.NewReplacer("{", "%7B", "}", "%7D")

// proxyLiteral returns s written as a literal for a Caddy JSON field that
// expands placeholders. User-supplied values are written as literals; input
// without braces is returned unchanged.
func proxyLiteral(s string) string {
	return placeholderDelimiterReplacer.Replace(s)
}

// proxyLiterals applies proxyLiteral to every entry and returns a new slice.
// A nil input returns nil.
func proxyLiterals(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = proxyLiteral(v)
	}
	return out
}
