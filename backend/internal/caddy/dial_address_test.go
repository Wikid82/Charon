package caddy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDial_AcceptedForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		dial string
		want DialAddress
	}{
		{"10.0.0.5:8080", DialAddress{Host: "10.0.0.5", PortStart: 8080, PortEnd: 8080}},
		{"tcp/127.0.0.1:2000-2100", DialAddress{Network: "tcp", Host: "127.0.0.1", PortStart: 2000, PortEnd: 2100}},
		{"TCP4/10.1.2.3:443", DialAddress{Network: "tcp4", Host: "10.1.2.3", PortStart: 443, PortEnd: 443}},
		{"tcp6/[fd12::5]:80", DialAddress{Network: "tcp6", Host: "fd12::5", PortStart: 80, PortEnd: 80}},
		{"[::1]:9000", DialAddress{Host: "::1", PortStart: 9000, PortEnd: 9000}},
		{"app.internal:65535", DialAddress{Host: "app.internal", PortStart: 65535, PortEnd: 65535}},
	}
	for _, tc := range cases {
		got, err := ParseDial(tc.dial)
		require.NoError(t, err, tc.dial)
		assert.Equal(t, tc.want, got, tc.dial)
	}
}

func TestParseDial_RefusedForms(t *testing.T) {
	t.Parallel()
	refused := []string{
		"{http.request.header.X}:80",
		"unix//app/data/run/caddy-admin.sock",
		"unixgram/run/x.sock",
		"unixpacket/run/x.sock",
		"fd/3",
		"fdgram/3",
		"udp/10.0.0.1:53",
		"/var/run/app.sock",
		"10.0.0.1",
		"10.0.0.1:0",
		"10.0.0.1:70000",
		"10.0.0.1:2100-2000",
		"10.0.0.1:abc",
		"10.0.0.1:1-2-3",
		":8080",
		"",
		"[::1:80",
	}
	for _, dial := range refused {
		_, err := ParseDial(dial)
		assert.True(t, errors.Is(err, ErrUnsupportedDial), "dial %q: got %v", dial, err)
	}
}

func TestParsePort_ErrorNamesTheValue(t *testing.T) {
	t.Parallel()
	_, err := parsePort("abc")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedDial)
}
