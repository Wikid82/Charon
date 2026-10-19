package handlers

import (
	"errors"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	codeInvalidIP        = "invalid_ip"
	codeInvalidDuration  = "invalid_duration"
	codeInvalidReason    = "invalid_reason"
	codeDecisionNotFound = "decision_not_found"

	msgIPRequired       = "ip is required"
	msgIPEmpty          = "ip cannot be empty"
	msgIPInvalid        = "ip must be a valid IPv4 or IPv6 address or a CIDR range"
	msgDurationInvalid  = "duration must be between 1m and 365d, for example 4h, 30m or 1d"
	msgReasonTooLong    = "reason must be at most 200 characters"
	msgIPTooBroad       = "that address range is too broad to ban"
	msgIPNotBannable    = "that address cannot be banned: unspecified and loopback addresses are not allowed"
	msgDecisionNotFound = "no decision with exactly this address or range"

	defaultBanDuration = "24h"
	minBanDuration     = time.Minute
	maxBanDuration     = 365 * 24 * time.Hour
	maxBanTargetLength = 64
	// minBanPrefixV4 and minBanPrefixV6 are the shortest prefix lengths a range ban accepts; a shorter prefix
	// would ban a large part of the internet. Private and LAN ranges are allowed on purpose.
	minBanPrefixV4 = 8
	minBanPrefixV6 = 16
	// mappedV4PrefixBits is the length of the IPv4-mapped IPv6 prefix (::ffff:0:0/96).
	mappedV4PrefixBits = 96
	maxBanDays         = 365
	maxBanReasonLength = 200
	maxLoggedCscliLine = 200
)

var (
	// banDayDuration is the whole-days form cscli accepts next to Go durations.
	banDayDuration = regexp.MustCompile(`^\d+d$`)
	// cscliDeletedCount matches the line cscli prints after "decisions delete".
	cscliDeletedCount = regexp.MustCompile(`(\d+) decision\(s\) deleted`)

	errBanTarget = errors.New("invalid ban target")
	// errBanTargetBroad marks a range wider than the limits above.
	errBanTargetBroad = errors.New("ban range too broad")
	// errBanTargetReserved marks an unspecified or loopback address or range.
	errBanTargetReserved = errors.New("ban target is unspecified or loopback")
)

// banTarget is a validated address or range, in the form cscli receives.
type banTarget struct {
	value   string
	isRange bool
}

// flag returns the cscli flag that takes this target: -i accepts only addresses, -r only ranges.
func (t banTarget) flag() string {
	if t.isRange {
		return "-r"
	}
	return "-i"
}

// parseBanTarget validates an address or range typed by the user. A range is returned in its masked form, and an
// IPv4-mapped IPv6 value in its IPv4 form. With enforcePolicy (banning), unspecified and loopback values and ranges
// wider than /8 (IPv4) or /16 (IPv6) are refused with errBanTargetReserved or errBanTargetBroad. Unbanning passes
// false so that an existing broad or loopback decision can still be removed. Any other bad value gets errBanTarget.
// Values that could be read as a cscli flag or that contain control characters or whitespace are refused.
func parseBanTarget(raw string, enforcePolicy bool) (banTarget, error) {
	s := strings.TrimSpace(raw)
	if s == "" || len(s) > maxBanTargetLength || s[0] == '-' {
		return banTarget{}, errBanTarget
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return banTarget{}, errBanTarget
		}
	}
	if addr, err := netip.ParseAddr(s); err == nil {
		if addr.Zone() != "" {
			return banTarget{}, errBanTarget
		}
		addr = addr.Unmap()
		if enforcePolicy && (addr.IsUnspecified() || addr.IsLoopback()) {
			return banTarget{}, errBanTargetReserved
		}
		return banTarget{value: addr.String()}, nil
	}
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return banTarget{}, errBanTarget
	}
	prefix, err = unmapPrefix(prefix)
	if err != nil {
		return banTarget{}, err
	}
	prefix = prefix.Masked()
	minBits := minBanPrefixV6
	if prefix.Addr().Is4() {
		minBits = minBanPrefixV4
	}
	if enforcePolicy && prefix.Bits() < minBits {
		return banTarget{}, errBanTargetBroad
	}
	if enforcePolicy && (prefix.Addr().IsUnspecified() || prefix.Addr().IsLoopback()) {
		return banTarget{}, errBanTargetReserved
	}
	return banTarget{value: prefix.String(), isRange: true}, nil
}

// unmapPrefix turns an IPv4-mapped IPv6 prefix into the IPv4 prefix. A mapped address with a prefix shorter than
// /96 covers more than the mapped space and is refused.
func unmapPrefix(p netip.Prefix) (netip.Prefix, error) {
	if !p.Addr().Is4In6() {
		return p, nil
	}
	if p.Bits() < mappedV4PrefixBits {
		return netip.Prefix{}, errBanTarget
	}
	return netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-mappedV4PrefixBits), nil
}

// parseBanDuration validates a ban duration: a Go duration or whole days ("1d"), from one minute to 365 days.
// The text is not changed, so cscli receives what the user typed.
func parseBanDuration(s string) (time.Duration, error) {
	var d time.Duration
	switch {
	case banDayDuration.MatchString(s):
		days, err := strconv.ParseUint(strings.TrimSuffix(s, "d"), 10, 32)
		if err != nil {
			return 0, errors.New("invalid duration")
		}
		// check the day count first: the multiplication below overflows int64 for very large counts
		if days > maxBanDays {
			return 0, errors.New("duration out of range")
		}
		d = time.Duration(days) * 24 * time.Hour
	case s != "" && (s[0] >= '0' && s[0] <= '9' || s[0] == '.'):
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return 0, errors.New("invalid duration")
		}
		d = parsed
	default:
		return 0, errors.New("invalid duration")
	}
	if d < minBanDuration || d > maxBanDuration {
		return 0, errors.New("duration out of range")
	}
	return d, nil
}

// cleanBanReason strips control characters and surrounding whitespace, then reports whether the result fits.
func cleanBanReason(s string) (string, bool) {
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s))
	return s, utf8.RuneCountInString(s) <= maxBanReasonLength
}

// respondInvalidBanTarget writes the coded 400 for a value that is not a usable address or range.
func respondInvalidBanTarget(c *gin.Context, raw string, enforcePolicy bool) {
	msg := msgIPInvalid
	_, err := parseBanTarget(raw, enforcePolicy)
	switch {
	case errors.Is(err, errBanTargetBroad):
		msg = msgIPTooBroad
	case errors.Is(err, errBanTargetReserved):
		msg = msgIPNotBannable
	case raw == "":
		msg = msgIPRequired
	case strings.TrimSpace(raw) == "":
		msg = msgIPEmpty
	}
	respondCoded(c, http.StatusBadRequest, codeInvalidIP, msg, nil)
}

// deletedDecisionCount reads the "N decision(s) deleted" count from cscli output. ok is false when the output
// carries no such line.
func deletedDecisionCount(out []byte) (n int, ok bool) {
	m := cscliDeletedCount.FindSubmatch(out)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0, false
	}
	return n, true
}

// firstCscliLine returns the first non-empty line of cscli output, sanitized and shortened for the log.
func firstCscliLine(out []byte) string {
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(util.SanitizeForLog(line)); line != "" {
			if len(line) > maxLoggedCscliLine {
				line = line[:maxLoggedCscliLine]
			}
			return line
		}
	}
	return ""
}
