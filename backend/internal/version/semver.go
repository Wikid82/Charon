package version

import (
	"regexp"

	"golang.org/x/mod/semver"
)

// canonicalPattern matches a strict SemVer 2.0.0 string: exactly three numeric
// components (no shorthand, no leading zeros), an optional prerelease, and an
// optional build-metadata suffix. Group 1 captures the part that survives
// canonicalization (core version plus prerelease, metadata stripped).
//
// Go's x/mod/semver accepts shorthand such as "v1.2", so the strict form is
// enforced here before any comparison is attempted.
var canonicalPattern = regexp.MustCompile(
	`^((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)` +
		`(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?)` +
		`(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`,
)

// Canonical returns v in bare MAJOR.MINOR.PATCH[-PRE] form with any build
// metadata removed. It returns ok=false when v is unversioned: "dev", empty,
// carries a leading "v", uses shorthand ("1", "1.2"), or is otherwise not
// strict SemVer. The regex only checks the grammar; x/mod/semver is the final
// authority so that edge cases it rejects (e.g. numeric prerelease identifiers
// with leading zeros) are rejected here too.
func Canonical(v string) (string, bool) {
	m := canonicalPattern.FindStringSubmatch(v)
	if m == nil {
		return "", false
	}
	if !semver.IsValid("v" + m[1]) {
		return "", false
	}
	return m[1], true
}

// IsUnversioned reports whether v cannot be used as a version anchor for
// comparisons. It is the negation of ok from Canonical.
func IsUnversioned(v string) bool {
	_, ok := Canonical(v)
	return !ok
}

// Compare orders two versions by SemVer precedence, ignoring build metadata.
// It returns -1, 0 or 1 as a is lower than, equal to, or higher than b. ok is
// false, and cmp is 0, when either side is unversioned (see Canonical).
func Compare(a, b string) (cmp int, ok bool) {
	ca, okA := Canonical(a)
	cb, okB := Canonical(b)
	if !okA || !okB {
		return 0, false
	}
	return semver.Compare("v"+ca, "v"+cb), true
}
