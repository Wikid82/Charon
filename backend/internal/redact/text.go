package redact

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MinValueLen is the shortest value Text replaces. Shorter values (switches such as "on" or "1") would
// mangle unrelated words, so they are left as they are.
const MinValueLen = 6

// truncateMarker marks a message cut by Truncate.
const truncateMarker = " [truncated]"

// credentialKeyPattern matches a credential key followed by its separator and a value, in plain
// ("api_key=value", "password: value") and JSON ("\"password\":\"value\"") text. Group 1 is the key and the
// separator, group 2 the value. A credential word must end the key, so token_path or password_policy do not
// match.
var credentialKeyPattern = regexp.MustCompile(
	`(?i)([A-Za-z0-9_.\-]*(?:password|passwd|pwd|secret|token|api_?key|private_key|passphrase|dsn)["']?\s*[=:]\s*["']?)([^\s"',;&}]+)`)

// schemeCredentialPattern matches a Bearer or Basic credential. Group 2 is the credential.
var schemeCredentialPattern = regexp.MustCompile(`(?i)(\b(?:bearer|basic)\s+)([A-Za-z0-9._~+/=-]+)`)

// urlUserinfoPattern matches the password of a URL with user info. Group 2 is the password.
var urlUserinfoPattern = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://[^\s:/@]+:)([^\s/@]+)@`)

// Text returns s with every known value and every credential-looking value replaced by Placeholder. The
// known values are replaced in their raw, JSON-escaped and Go-quoted forms, longest first, so a value that
// contains another is replaced whole. The patterns cover text whose values Charon did not send: a
// credential key followed by a value, a Bearer or Basic credential, and the password of a URL with user info.
// Values shorter than MinValueLen are ignored.
func Text(s string, known []string) string {
	s = replaceKnownValues(s, known)
	s = replaceGroupTwo(s, schemeCredentialPattern)
	s = replaceGroupTwo(s, urlUserinfoPattern)
	return replaceGroupTwo(s, credentialKeyPattern)
}

// replaceKnownValues replaces every form of every known value that is long enough.
func replaceKnownValues(s string, known []string) string {
	forms := map[string]struct{}{}
	for _, v := range known {
		if len(v) < MinValueLen {
			continue
		}
		for _, f := range valueForms(v) {
			forms[f] = struct{}{}
		}
	}
	list := make([]string, 0, len(forms))
	for f := range forms {
		list = append(list, f)
	}
	sort.Slice(list, func(i, j int) bool {
		if len(list[i]) != len(list[j]) {
			return len(list[i]) > len(list[j])
		}
		return list[i] < list[j]
	})
	for _, f := range list {
		s = strings.ReplaceAll(s, f, Placeholder)
	}
	return s
}

// valueForms returns the raw value and its forms inside a JSON string (with and without HTML escaping) and
// inside a Go quoted string, without the surrounding quotes.
func valueForms(v string) []string {
	return []string{
		v,
		jsonInner(v, true),
		jsonInner(v, false),
		strconv.Quote(v)[1 : len(strconv.Quote(v))-1],
	}
}

// jsonInner returns v as it appears inside a JSON string, without the surrounding quotes.
func jsonInner(v string, escapeHTML bool) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(escapeHTML)
	if err := enc.Encode(v); err != nil {
		return v
	}
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(buf.String()), `"`), `"`)
}

// replaceGroupTwo replaces the second group of every match of re with Placeholder when that group is at
// least MinValueLen bytes long. The first group and the rest of the match are kept.
func replaceGroupTwo(s string, re *regexp.Regexp) string {
	matches := re.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[4], m[5]
		if end-start < MinValueLen {
			continue
		}
		b.WriteString(s[last:start])
		b.WriteString(Placeholder)
		last = end
	}
	b.WriteString(s[last:])
	return b.String()
}

// Truncate cuts s to at most n bytes, on a rune boundary, and marks the cut with " [truncated]". The marker
// is added after the cut, so the result can be longer than n by the marker length.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + truncateMarker
}
