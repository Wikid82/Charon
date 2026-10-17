package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestParseBanTarget(t *testing.T) {
	t.Parallel()
	ok := map[string]banTarget{
		"192.0.2.10":            {value: "192.0.2.10"},
		"2001:DB8::1":           {value: "2001:db8::1"},
		"192.0.2.77/24":         {value: "192.0.2.0/24", isRange: true},
		"2001:db8:1:2::9/48":    {value: "2001:db8:1::/48", isRange: true},
		"198.51.100.0/32":       {value: "198.51.100.0/32", isRange: true},
		"::ffff:192.0.2.1":      {value: "192.0.2.1"},
		"::ffff:192.0.2.0/120":  {value: "192.0.2.0/24", isRange: true},
		"::ffff:192.0.2.77/120": {value: "192.0.2.0/24", isRange: true},
		"::ffff:1.0.0.0/104":    {value: "1.0.0.0/8", isRange: true},
		"10.0.0.0/8":            {value: "10.0.0.0/8", isRange: true},
		"192.168.1.0/24":        {value: "192.168.1.0/24", isRange: true},
		"192.168.1.5":           {value: "192.168.1.5"},
		"1.0.0.0/8":             {value: "1.0.0.0/8", isRange: true},
		"2001:db8::/16":         {value: "2001::/16", isRange: true},
		"fd12:3456::/32":        {value: "fd12:3456::/32", isRange: true},
	}
	for in, want := range ok {
		got, err := parseBanTarget(in, true)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "  ", "-1.2.3.4", "fe80::1%eth0", "1.2.3.4/33", "a b", "1.2.3.4\u00a05", "1.2.3.4\x00", strings.Repeat("1", 65), "x"} {
		_, err := parseBanTarget(in, true)
		assert.ErrorIs(t, err, errBanTarget, in)
	}
	reserved := []string{"0.0.0.0", "::", "127.0.0.1", "127.255.255.254", "::1", "::ffff:127.0.0.1", "::ffff:0.0.0.0", "127.0.0.0/8", "127.0.0.1/32", "0.0.0.0/8", "::1/128", "::/64", "::ffff:127.0.0.0/104"}
	for _, in := range reserved {
		_, err := parseBanTarget(in, true)
		assert.ErrorIs(t, err, errBanTargetReserved, in)
	}
	broad := []string{"0.0.0.0/0", "::/0", "1.0.0.0/7", "128.0.0.0/1", "2001:db8::/15", "::ffff:0.0.0.0/96", "::ffff:0:0/96"}
	for _, in := range broad {
		_, err := parseBanTarget(in, true)
		assert.Error(t, err, in)
		assert.False(t, errors.Is(err, errBanTarget), in)
	}
	for _, in := range []string{"0.0.0.0/0", "::/0", "1.0.0.0/7", "2001:db8::/15"} {
		_, err := parseBanTarget(in, true)
		assert.ErrorIs(t, err, errBanTargetBroad, in)
	}
	// a mapped address with a prefix shorter than /96 is nonsensical
	_, err := parseBanTarget("::ffff:192.0.2.0/95", true)
	assert.ErrorIs(t, err, errBanTarget)
	assert.Equal(t, "-i", banTarget{value: "192.0.2.1"}.flag())
	assert.Equal(t, "-r", banTarget{value: "192.0.2.0/24", isRange: true}.flag())
}

// TestParseBanTarget_UnbanSkipsPolicy: unbanning must reach existing broad or loopback decisions.
func TestParseBanTarget_UnbanSkipsPolicy(t *testing.T) {
	t.Parallel()
	want := map[string]banTarget{
		"0.0.0.0/0":         {value: "0.0.0.0/0", isRange: true},
		"::/0":              {value: "::/0", isRange: true},
		"1.0.0.0/7":         {value: "0.0.0.0/7", isRange: true},
		"2001:db8::/15":     {value: "2000::/15", isRange: true},
		"127.0.0.1":         {value: "127.0.0.1"},
		"::ffff:127.0.0.1":  {value: "127.0.0.1"},
		"0.0.0.0":           {value: "0.0.0.0"},
		"::":                {value: "::"},
		"::1":               {value: "::1"},
		"127.0.0.0/8":       {value: "127.0.0.0/8", isRange: true},
		"::ffff:0.0.0.0/96": {value: "0.0.0.0/0", isRange: true},
		"192.0.2.77/24":     {value: "192.0.2.0/24", isRange: true},
	}
	for in, w := range want {
		got, err := parseBanTarget(in, false)
		require.NoError(t, err, in)
		assert.Equal(t, w, got, in)
	}
	// injection and syntax rules still apply
	for _, in := range []string{"", "  ", "-0.0.0.0/0", "fe80::1%eth0", "1.2.3.4/33", "a b", "1.2.3.4\x00", strings.Repeat("1", 65), "::ffff:192.0.2.0/95"} {
		_, err := parseBanTarget(in, false)
		assert.ErrorIs(t, err, errBanTarget, in)
	}
}

func TestParseBanDuration(t *testing.T) {
	t.Parallel()
	ok := map[string]time.Duration{
		"1m": time.Minute, "364d": 364 * 24 * time.Hour, "90s": 90 * time.Second, "2.5h": 150 * time.Minute, "1d": 24 * time.Hour,
		"365d": 365 * 24 * time.Hour, "8760h": 8760 * time.Hour, "1h30m": 90 * time.Minute, ".5h": 30 * time.Minute,
	}
	for in, want := range ok {
		got, err := parseBanDuration(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "0", "59s", "366d", "8761h", "+4h", "-4h", "4H", "1w", "1h1d", "4 h", "d", "99999999999d", "1e3h", "0d", "213504d", "4294967295d", "4294967296d"} {
		_, err := parseBanDuration(in)
		assert.Error(t, err, in)
	}
}

func TestCleanBanReason(t *testing.T) {
	t.Parallel()
	got, ok := cleanBanReason(" a\nb\x00\x1b[0m\u0085c ")
	assert.True(t, ok)
	assert.Equal(t, "ab[0mc", got)

	_, ok = cleanBanReason(strings.Repeat("é", 200))
	assert.True(t, ok)
	_, ok = cleanBanReason(strings.Repeat("é", 201))
	assert.False(t, ok)
	// control characters do not count toward the cap
	_, ok = cleanBanReason(strings.Repeat("a", 200) + "\n\n")
	assert.True(t, ok)
}

func TestDeletedDecisionCount(t *testing.T) {
	t.Parallel()
	n, ok := deletedDecisionCount([]byte("level=info msg=\"3 decision(s) deleted\"\n"))
	assert.True(t, ok)
	assert.Equal(t, 3, n)
	n, ok = deletedDecisionCount([]byte("level=info msg=\"0 decision(s) deleted\""))
	assert.True(t, ok)
	assert.Zero(t, n)
	_, ok = deletedDecisionCount([]byte("nothing here"))
	assert.False(t, ok)
	_, ok = deletedDecisionCount([]byte("99999999999999999999999 decision(s) deleted"))
	assert.False(t, ok)
}

func TestFirstCscliLine(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Error: bad [0m", firstCscliLine([]byte("\n  \nError: bad\x1b[0m\nsecond")))
	assert.Empty(t, firstCscliLine(nil))
	assert.Len(t, firstCscliLine([]byte(strings.Repeat("x", 500))), maxLoggedCscliLine)
}

func TestBanIP_BadBodyAndMissingIPAreCoded(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{"invalid json": "invalid json", "missing ip": "{}"} {
		w := pr4Serve(pr4Router(pr4Handler(t, &pr4Exec{}, t.TempDir())), http.MethodPost, pr4BanURL, body)
		require.Equal(t, http.StatusBadRequest, w.Code, name)
		assert.Equal(t, "ip is required", pr4Body(t, w)["error"], name)
	}
}

func TestBanIP_AuditFailureDoesNotFailTheBan(t *testing.T) {
	t.Parallel()
	h := pr4Handler(t, pr4Answer(t, "decisions_add_ban"), t.TempDir())
	require.NoError(t, h.DB.Migrator().DropTable(&models.SecurityDecision{}))
	w := pr4Serve(pr4Router(h), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "1h", ""))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestUnbanIP_WithoutCountLineIsASuccess(t *testing.T) {
	t.Parallel()
	h := pr4Handler(t, &pr4Exec{}, t.TempDir())
	w := pr4Serve(pr4Router(h), http.MethodDelete, pr4BanURL+"/192.0.2.9", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, pr4Body(t, w), "deleted")
}

func TestRespondInvalidBanTarget_Messages(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"0.0.0.0/0":  msgIPTooBroad,
		"::/0":       msgIPTooBroad,
		"0.0.0.0":    msgIPNotBannable,
		"127.0.0.1":  msgIPNotBannable,
		"notanip":    msgIPInvalid,
		"":           msgIPRequired,
		"   ":        msgIPEmpty,
		"1.0.0.0/7":  msgIPTooBroad,
		"::1":        msgIPNotBannable,
		"2001::/15":  msgIPTooBroad,
		"1.2.3.4/33": msgIPInvalid,
	}
	for in, want := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		respondInvalidBanTarget(c, in, true)
		require.Equal(t, http.StatusBadRequest, w.Code, in)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, codeInvalidIP, body["error_code"], in)
		assert.Equal(t, want, body["error"], in)
	}
}

func TestBanRoute_RefusesBroadAndReservedTargets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ ip, want string }{
		{"0.0.0.0/0", msgIPTooBroad},
		{"::/0", msgIPTooBroad},
		{"1.0.0.0/7", msgIPTooBroad},
		{"127.0.0.1", msgIPNotBannable},
		{"::", msgIPNotBannable},
	} {
		ex := &pr4Exec{}
		r := pr4Router(pr4Handler(t, ex, t.TempDir()))
		w := pr4Serve(r, http.MethodPost, pr4BanURL, `{"ip":"`+tc.ip+`","duration":"1h"}`)
		require.Equal(t, http.StatusBadRequest, w.Code, tc.ip)
		body := pr4Body(t, w)
		assert.Equal(t, "invalid_ip", body["error_code"], tc.ip)
		assert.Equal(t, tc.want, body["error"], tc.ip)
		assert.Equal(t, 0, ex.callCount(), "cscli is not run for "+tc.ip)
	}
}

func TestUnbanRoute_RemovesBroadAndReservedDecisions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ ip, flag, value string }{
		{"0.0.0.0/0", "-r", "0.0.0.0/0"},
		{"::/0", "-r", "::/0"},
		{"1.0.0.0/7", "-r", "0.0.0.0/7"},
		{"127.0.0.1", "-i", "127.0.0.1"},
		{"::ffff:127.0.0.1", "-i", "127.0.0.1"},
		{"::1", "-i", "::1"},
	} {
		ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) {
			return []byte("level=info msg=\"1 decision(s) deleted\"\n"), nil
		}}
		r := pr4Router(pr4Handler(t, ex, t.TempDir()))
		w := pr4Serve(r, http.MethodDelete, pr4BanURL+"/"+url.PathEscape(tc.ip), "")
		require.Equal(t, http.StatusOK, w.Code, tc.ip+" "+w.Body.String())
		assert.Equal(t, []string{"decisions", "delete", tc.flag, tc.value}, pr4StripConfig(ex.lastCall(t)), tc.ip)
	}
}

func TestUnbanRoute_StillRefusesInjection(t *testing.T) {
	t.Parallel()
	for _, ip := range []string{"-0.0.0.0", "fe80::1%25eth0", "1.2.3.4%00"} {
		ex := &pr4Exec{}
		r := pr4Router(pr4Handler(t, ex, t.TempDir()))
		w := pr4Serve(r, http.MethodDelete, pr4BanURL+"/"+ip, "")
		require.Equal(t, http.StatusBadRequest, w.Code, ip)
		assert.Equal(t, msgIPInvalid, pr4Body(t, w)["error"], ip)
		assert.Equal(t, 0, ex.callCount(), ip)
	}
}

func TestBanRoute_MappedAddressIsBannedInItsIPv4Form(t *testing.T) {
	t.Parallel()
	ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) {
		return []byte("level=info msg=\"Decision successfully added\"\n"), nil
	}}
	r := pr4Router(pr4Handler(t, ex, t.TempDir()))
	w := pr4Serve(r, http.MethodPost, pr4BanURL, `{"ip":"::ffff:192.0.2.1","duration":"1h"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	call := pr4StripConfig(ex.lastCall(t))
	flag, _ := pr4Flag(call, "-i")
	assert.Equal(t, "192.0.2.1", flag)
}
