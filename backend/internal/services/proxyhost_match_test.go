package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestNormalizeHostName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "example.com", want: "example.com"},
		{in: "  Example.COM  ", want: "example.com"},
		{in: "example.com.", want: "example.com"},
		{in: "example.com:8443", want: "example.com"},
		{in: "example.com.:443", want: "example.com"},
		{in: "my_host.internal", want: "my_host.internal"},
		{in: "10.0.0.5", want: "10.0.0.5"},
		{in: "10.0.0.5:8080", want: "10.0.0.5"},
		{in: "[::1]", want: "::1"},
		{in: "[::1]:8080", want: "::1"},
		{in: "::1", want: "::1"},
		{in: "[2001:DB8::1]:443", want: "2001:db8::1"},
		{in: "::ffff:10.0.0.5", want: "10.0.0.5"},
		{in: "bücher.example", want: "xn--bcher-kva.example"},
		{in: "a\u0378.example", wantErr: true},
		{in: "example.com:", want: "example.com"},
		{in: "", wantErr: true},
		{in: "   ", wantErr: true},
		{in: ".", wantErr: true},
		{in: "%", wantErr: true},
		{in: "%.example.com", wantErr: true},
		{in: "*.example.com", wantErr: true},
		{in: "exa mple.com", wantErr: true},
		{in: "example.com\n", want: "example.com"},
		{in: "exa\x00mple.com", wantErr: true},
		{in: "exa\tmple.com", wantErr: true},
		{in: "fe80::1%eth0", wantErr: true},
		{in: "[::1", wantErr: true},
		{in: "a:b:c", wantErr: true},
		{in: "[::1]x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeHostName(tt.in)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidHostName)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchProxyHosts(t *testing.T) {
	t.Parallel()
	hosts := []models.ProxyHost{
		{ID: 1, DomainNames: "example.com, www.example.com"},
		{ID: 2, DomainNames: "*.example.com"},
		{ID: 3, DomainNames: "api.example.com"},
		{ID: 4, DomainNames: "notexample.com"},
		{ID: 5, DomainNames: "My_Host.Internal:8443"},
		{ID: 6, DomainNames: "10.0.0.5"},
		{ID: 7, DomainNames: "::1"},
		{ID: 8, DomainNames: "bücher.example"},
		{ID: 9, DomainNames: "*.wild.test"},
		{ID: 10, DomainNames: "bad host, listed.test"},
	}
	ids := func(hs []models.ProxyHost) []uint {
		out := make([]uint, 0, len(hs))
		for _, h := range hs {
			out = append(out, h.ID)
		}
		return out
	}

	tests := []struct {
		name string
		host string
		want []uint
	}{
		{"exact", "example.com", []uint{1}},
		{"second entry in list", "www.example.com", []uint{1}},
		{"uppercase and port", "EXAMPLE.com:8443", []uint{1}},
		{"trailing dot", "example.com.", []uint{1}},
		{"near miss prefix", "notexample.com", []uint{4}},
		{"near miss suffix", "example.com.evil.test", nil},
		{"suffix of stored name", "ample.com", nil},
		{"exact beats wildcard", "api.example.com", []uint{3}},
		{"wildcard one label", "a.example.com", []uint{2}},
		{"wildcard two labels", "a.b.example.com", nil},
		{"wildcard base itself", "wild.test", nil},
		{"wildcard only host", "x.wild.test", []uint{9}},
		{"underscore name", "my_host.internal", []uint{5}},
		{"ipv4", "10.0.0.5:80", []uint{6}},
		{"ipv6 literal", "[::1]:8080", []uint{7}},
		{"idna", "BÜCHER.example", []uint{8}},
		{"unknown", "other.test", nil},
		{"unusable stored entry is skipped", "listed.test", []uint{10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := MatchProxyHosts(hosts, tt.host)
			require.NoError(t, err)
			if tt.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, ids(got))
		})
	}

	for _, bad := range []string{"%", "%example%", "_", "*", "", "a b"} {
		t.Run("rejected "+bad, func(t *testing.T) {
			t.Parallel()
			got, err := MatchProxyHosts(hosts, bad)
			if bad == "_" {
				// Underscore is a legal name character; it simply matches nothing here.
				require.NoError(t, err)
				assert.Empty(t, got)
				return
			}
			assert.ErrorIs(t, err, ErrInvalidHostName)
			assert.Empty(t, got)
		})
	}
}

func TestFindProxyHostsByDomain(t *testing.T) {
	t.Parallel()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}))
	require.NoError(t, db.Create(&models.ProxyHost{UUID: "u1", Name: "a", DomainNames: "app.example.com", ForwardAuthEnabled: true}).Error)

	got, err := FindProxyHostsByDomain(db, "app.example.com:443")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].ForwardAuthEnabled)

	got, err = FindProxyHostsByDomain(db, "example.com")
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = FindProxyHostsByDomain(db, "%")
	assert.ErrorIs(t, err, ErrInvalidHostName)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = FindProxyHostsByDomain(db, "app.example.com")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidHostName)
}
