package redact

import (
	"regexp"
	"strings"
)

// urlSchemePattern is the scheme syntax of RFC 3986.
var urlSchemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)

// URLPassword replaces the password in the userinfo of "scheme://user:password@host/..." with placeholder.
// It returns the new text and the password it removed. ok is false when v is not such a URL, when its
// password is empty or already the placeholder, or when keep reports that the password must be left alone
// (for example an environment reference). keep may be nil.
func URLPassword(v, placeholder string, keep func(password string) bool) (redacted, password string, ok bool) {
	scheme, rest, found := strings.Cut(v, "://")
	if !found || !urlSchemePattern.MatchString(scheme) {
		return v, "", false
	}
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		end = len(rest)
	}
	authority := rest[:end]
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return v, "", false
	}
	user, password, hasPassword := strings.Cut(authority[:at], ":")
	if !hasPassword || password == "" || password == placeholder || (keep != nil && keep(password)) {
		return v, "", false
	}
	return scheme + "://" + user + ":" + placeholder + authority[at:] + rest[end:], password, true
}
