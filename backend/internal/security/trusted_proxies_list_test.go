package security

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTrustedProxies_Canonicalises(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"bare IPv4 becomes /32", []string{"172.18.0.5"}, []string{"172.18.0.5/32"}},
		{"bare IPv6 becomes /128", []string{"fd00::5"}, []string{"fd00::5/128"}},
		{"CIDR is masked to its network", []string{"172.18.9.7/16"}, []string{"172.18.0.0/16"}},
		{"IPv4-mapped address is unmapped", []string{"::ffff:172.18.0.5"}, []string{"172.18.0.5/32"}},
		{"IPv4-mapped CIDR is unmapped", []string{"::ffff:203.0.113.0/120"}, []string{"203.0.113.0/24"}},
		{"whitespace and empty entries are dropped", []string{"  10.1.2.3  ", "", "   "}, []string{"10.1.2.3/32"}},
		{"duplicates merge and sort", []string{"10.0.0.2", "10.0.0.1", "10.0.0.2/32"}, []string{"10.0.0.1/32", "10.0.0.2/32"}},
		{"loopback is never stored", []string{"127.0.0.1", "::1", "172.18.0.5"}, []string{"172.18.0.5/32"}},
		{"empty list stores nothing", nil, []string{}},
		{"IPv6 /24 is masked to its network", []string{"2001:db8::/24"}, []string{"2001:d00::/24"}},
		{"IPv4 /16 is not flagged", []string{"198.51.0.0/16"}, []string{"198.51.0.0/16"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings, err := NormalizeTrustedProxies(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Empty(t, warnings, "no warnings expected for %v", tt.input)
		})
	}
}

func TestNormalizeTrustedProxies_Refusals(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		wantKind  error
		wantEntry string
	}{
		{"unparsable", []string{"not-an-ip"}, ErrTrustedProxyInvalid, "not-an-ip"},
		{"unparsable CIDR", []string{"10.0.0.0/33"}, ErrTrustedProxyInvalid, "10.0.0.0/33"},
		{"zero prefix", []string{"0.0.0.0/0"}, ErrTrustedProxyTooBroad, "0.0.0.0/0"},
		{"IPv4 /7 is wider than /8", []string{"10.0.0.0/7"}, ErrTrustedProxyTooBroad, "10.0.0.0/7"},
		{"IPv6 /23 is wider than /24", []string{"2001:db8::/23"}, ErrTrustedProxyTooBroad, "2001:db8::/23"},
		{"IPv4-mapped zero range", []string{"::ffff:0:0/96"}, ErrTrustedProxyTooBroad, "::ffff:0:0/96"},
		{"invalid entry after a valid one", []string{"10.0.0.1", "bogus"}, ErrTrustedProxyInvalid, "bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings, err := NormalizeTrustedProxies(tt.input)
			require.Error(t, err)
			assert.Nil(t, got)
			assert.Nil(t, warnings)
			assert.ErrorIs(t, err, tt.wantKind)
			var entryErr *TrustedProxyEntryError
			require.True(t, errors.As(err, &entryErr))
			assert.Equal(t, tt.wantEntry, entryErr.Entry)
			assert.Contains(t, err.Error(), tt.wantEntry)
		})
	}
}

func TestNormalizeTrustedProxies_TooMany(t *testing.T) {
	var entries []string
	for i := range MaxTrustedProxies + 1 {
		entries = append(entries, "172.18."+strconv.Itoa(i/200)+"."+strconv.Itoa(i%200)+"/32")
	}
	_, _, err := NormalizeTrustedProxies(entries)
	assert.ErrorIs(t, err, ErrTooManyTrustedProxies)

	kept, _, err := NormalizeTrustedProxies(entries[:MaxTrustedProxies])
	require.NoError(t, err)
	assert.Len(t, kept, MaxTrustedProxies)
}

func TestNormalizeTrustedProxies_Warnings(t *testing.T) {
	t.Run("entry covering a local-network range warns", func(t *testing.T) {
		got, warnings, err := NormalizeTrustedProxies([]string{"10.0.0.0/8"})
		require.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.0/8"}, got)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "local-network range 10.0.0.0/8")
	})
	t.Run("entry inside a local-network range is not flagged", func(t *testing.T) {
		_, warnings, err := NormalizeTrustedProxies([]string{"10.1.0.0/16"})
		require.NoError(t, err)
		assert.Empty(t, warnings)
	})
	t.Run("IPv4 /8 is accepted with a wide-range warning", func(t *testing.T) {
		got, warnings, err := NormalizeTrustedProxies([]string{"198.0.0.0/8"})
		require.NoError(t, err)
		assert.Equal(t, []string{"198.0.0.0/8"}, got)
		require.Len(t, warnings, 1)
	})
	t.Run("IPv4 range wider than /16 warns", func(t *testing.T) {
		_, warnings, err := NormalizeTrustedProxies([]string{"198.51.0.0/12"})
		require.NoError(t, err)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "wide range")
	})
	t.Run("duplicate of a warned range warns once", func(t *testing.T) {
		_, warnings, err := NormalizeTrustedProxies([]string{"10.0.0.0/8", "10.0.0.0/8"})
		require.NoError(t, err)
		assert.Len(t, warnings, 1)
	})
}

func TestClampTrustedProxies(t *testing.T) {
	kept, dropped := ClampTrustedProxies([]string{"172.18.0.5", "bogus", "0.0.0.0/0", "10.0.0.0/8", "127.0.0.1", "172.18.0.5"})
	assert.Equal(t, []string{"10.0.0.0/8", "172.18.0.5/32"}, kept)
	assert.Equal(t, []string{"bogus", "0.0.0.0/0"}, dropped)

	var many []string
	for i := range MaxTrustedProxies + 2 {
		many = append(many, "172.18."+strconv.Itoa(i/200)+"."+strconv.Itoa(i%200))
	}
	kept, dropped = ClampTrustedProxies(many)
	assert.Len(t, kept, MaxTrustedProxies)
	assert.Len(t, dropped, 2, "entries past the limit are dropped")
}

func TestEffectiveTrustedProxies(t *testing.T) {
	stored := func(v string) *string { return &v }
	t.Run("saved value is used alone", func(t *testing.T) {
		trust := EffectiveTrustedProxies(stored("172.18.0.5/32"), []string{"10.9.9.9"})
		assert.Equal(t, "setting", trust.Source)
		assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "172.18.0.5/32"}, trust.Ranges)
		assert.Equal(t, []string{"172.18.0.5/32"}, trust.Configured)
		assert.NoError(t, trust.StoredError)
	})
	t.Run("empty saved value trusts loopback only", func(t *testing.T) {
		trust := EffectiveTrustedProxies(stored(""), []string{"10.9.9.9"})
		assert.Equal(t, "setting", trust.Source)
		assert.Equal(t, AlwaysTrustedProxies(), trust.Ranges)
		assert.Empty(t, trust.Configured)
	})
	t.Run("invalid saved value falls back to loopback and reports the error", func(t *testing.T) {
		trust := EffectiveTrustedProxies(stored("172.18.0.5/32,0.0.0.0/0"), nil)
		assert.Equal(t, "setting", trust.Source)
		assert.Equal(t, AlwaysTrustedProxies(), trust.Ranges)
		assert.ErrorIs(t, trust.StoredError, ErrTrustedProxyTooBroad)
		assert.NotEmpty(t, trust.Dropped)
	})
	t.Run("environment list is clamped when nothing is saved", func(t *testing.T) {
		trust := EffectiveTrustedProxies(nil, []string{" ", "10.0.0.0/8", "0.0.0.0/0"})
		assert.Equal(t, "environment", trust.Source)
		assert.Equal(t, []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8"}, trust.Ranges)
		assert.Equal(t, []string{"0.0.0.0/0"}, trust.Dropped)
	})
	t.Run("environment with only refused entries trusts loopback", func(t *testing.T) {
		trust := EffectiveTrustedProxies(nil, []string{"bogus"})
		assert.Equal(t, "environment", trust.Source)
		assert.Equal(t, AlwaysTrustedProxies(), trust.Ranges)
		assert.Equal(t, []string{"bogus"}, trust.Dropped)
	})
	t.Run("no saved value and no environment is the default", func(t *testing.T) {
		trust := EffectiveTrustedProxies(nil, nil)
		assert.Equal(t, "default", trust.Source)
		assert.Equal(t, AlwaysTrustedProxies(), trust.Ranges)
	})
}

func TestAlwaysTrustedProxies_ReturnsFreshSlice(t *testing.T) {
	first := AlwaysTrustedProxies()
	first[0] = "changed"
	assert.Equal(t, "127.0.0.1/32", AlwaysTrustedProxies()[0])
}
