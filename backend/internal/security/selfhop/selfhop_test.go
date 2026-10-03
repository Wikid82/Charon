package selfhop

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecret_UniqueAndVerifies(t *testing.T) {
	a, err := NewSecret()
	require.NoError(t, err)
	b, err := NewSecret()
	require.NoError(t, err)

	assert.NotEqual(t, a.Reveal(), b.Reveal())
	assert.GreaterOrEqual(t, len(a.Reveal()), minSecretLength)
	assert.True(t, a.Verify(a.Reveal()))
	assert.False(t, a.Verify(b.Reveal()))
	assert.False(t, a.Verify(""))
	assert.False(t, a.Verify(a.Reveal()+"x"))
}

func TestParseSecret(t *testing.T) {
	_, err := ParseSecret("short")
	require.Error(t, err)

	s, err := ParseSecret(strings.Repeat("a", minSecretLength))
	require.NoError(t, err)
	assert.True(t, s.Verify(strings.Repeat("a", minSecretLength)))
}

func TestSecret_NilAndEmptyNeverVerify(t *testing.T) {
	var nilSecret *Secret
	assert.False(t, nilSecret.Verify("anything"))
	assert.Equal(t, "", nilSecret.Reveal())
	assert.False(t, (&Secret{}).Verify(""))
	assert.False(t, (&Secret{}).Verify("x"))
}

func TestSecret_FormattingIsRedacted(t *testing.T) {
	s, err := NewSecret()
	require.NoError(t, err)
	raw := s.Reveal()

	type holder struct{ S *Secret }
	outputs := []string{
		s.String(),
		s.GoString(),
		fmt.Sprintf("%v", s),
		fmt.Sprintf("%+v", s),
		fmt.Sprintf("%#v", s),
		fmt.Sprintf("%s", s), //nolint:gosimple // exercising the verb
		fmt.Sprintf("%v", holder{S: s}),
		fmt.Sprintf("%+v", holder{S: s}),
	}
	enc, err := json.Marshal(holder{S: s})
	require.NoError(t, err)
	outputs = append(outputs, string(enc))

	for _, out := range outputs {
		assert.NotContains(t, out, raw)
		assert.Contains(t, out, redacted)
	}
}

func fakeLister(addrs ...net.Addr) func() ([]net.Addr, error) {
	return func() ([]net.Addr, error) { return addrs, nil }
}

func ipnet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, n, err := net.ParseCIDR(cidr)
	require.NoError(t, err)
	n.IP = ip
	return n
}

func TestLocalAddrs_Contains(t *testing.T) {
	l := NewLocalAddrs()
	l.lister = fakeLister(ipnet(t, "172.18.0.5/16"), &net.IPAddr{IP: net.ParseIP("10.1.2.3")})

	assert.True(t, l.Contains(net.ParseIP("127.0.0.1")))
	assert.True(t, l.Contains(net.ParseIP("::1")))
	assert.True(t, l.Contains(net.ParseIP("172.18.0.5")))
	assert.True(t, l.Contains(net.ParseIP("10.1.2.3")))
	assert.False(t, l.Contains(net.ParseIP("172.18.0.6")))
	assert.False(t, l.Contains(net.ParseIP("203.0.113.9")))
	assert.False(t, l.Contains(nil))
}

func TestLocalAddrs_CacheAndRefresh(t *testing.T) {
	calls := 0
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLocalAddrs()
	l.now = func() time.Time { return now }
	l.lister = func() ([]net.Addr, error) {
		calls++
		return []net.Addr{ipnet(t, "10.9.9.9/24")}, nil
	}

	assert.True(t, l.Contains(net.ParseIP("10.9.9.9")))
	assert.True(t, l.Contains(net.ParseIP("10.9.9.9")))
	assert.Equal(t, 1, calls)

	now = now.Add(defaultLocalAddrsTTL + time.Second)
	assert.True(t, l.Contains(net.ParseIP("10.9.9.9")))
	assert.Equal(t, 2, calls)
}

func TestLocalAddrs_ListerErrorKeepsPreviousSnapshot(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fail := false
	l := NewLocalAddrs()
	l.now = func() time.Time { return now }
	l.lister = func() ([]net.Addr, error) {
		if fail {
			return nil, errors.New("boom")
		}
		return []net.Addr{ipnet(t, "10.9.9.9/24")}, nil
	}
	assert.True(t, l.Contains(net.ParseIP("10.9.9.9")))

	fail = true
	now = now.Add(time.Hour)
	assert.True(t, l.Contains(net.ParseIP("10.9.9.9")))
	assert.False(t, l.Contains(net.ParseIP("10.9.9.10")))
}

func TestLocalAddrs_ListerErrorWithoutSnapshotDeniesNonLoopback(t *testing.T) {
	l := NewLocalAddrs()
	l.lister = func() ([]net.Addr, error) { return nil, errors.New("boom") }
	assert.False(t, l.Contains(net.ParseIP("10.9.9.9")))
	assert.True(t, l.Contains(net.ParseIP("127.0.0.1")))
}

func TestLocalAddrs_IsLocalHost(t *testing.T) {
	t.Setenv("HOSTNAME", "Charon-Box")
	l := NewLocalAddrs()
	l.lister = fakeLister(ipnet(t, "172.18.0.5/16"))

	for _, host := range []string{"localhost", "LOCALHOST", "127.0.0.1", "::1", "[::1]", "0.0.0.0", "::", "172.18.0.5", "charon-box", "localhost."} {
		assert.True(t, l.IsLocalHost(host), host)
	}
	for _, host := range []string{"", "example.com", "172.18.0.6", "203.0.113.1", "charon-box.evil.example", "orthrus:abc"} {
		assert.False(t, l.IsLocalHost(host), host)
	}
}

func TestNewSecret_EntropyFailure(t *testing.T) {
	orig := randRead
	randRead = func([]byte) (int, error) { return 0, errors.New("no entropy") }
	t.Cleanup(func() { randRead = orig })

	s, err := NewSecret()
	require.Error(t, err)
	assert.Nil(t, s)
}
