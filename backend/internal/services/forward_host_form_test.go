package services

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// forwardHostFormCases are forward host values with the expected outcome of the
// form check. Accepted values are a host, an IPv4 or bracketed IPv6 address, an
// optional port, a docker-style name, or the same with an http:// or https://
// prefix. Anything with a path, query, fragment, user-info or whitespace is refused.
var forwardHostFormCases = []struct {
	name    string
	raw     string
	refused bool
}{
	{name: "hostname", raw: "example.com"},
	{name: "hostname with port", raw: "example.com:3000"},
	{name: "ipv4", raw: "192.168.1.10"},
	{name: "ipv4 with port", raw: "192.168.1.10:8080"},
	{name: "bracketed ipv6 with port", raw: "[::1]:8080"},
	{name: "docker container name", raw: "my_app-1"},
	{name: "docker container name with port", raw: "my-app:8080"},
	{name: "http scheme", raw: "http://example.com:8080"},
	{name: "https scheme", raw: "https://example.com"},
	{name: "https scheme with bracketed ipv6", raw: "https://[::1]:443"},
	{name: "unix socket style", raw: "unix//x", refused: true},
	{name: "unix socket absolute style", raw: "unix//app/data/run/caddy-admin.sock", refused: true},
	{name: "single slash", raw: "a/b", refused: true},
	{name: "traversal segment", raw: "host/../x", refused: true},
	{name: "path after host", raw: "example.com/api", refused: true},
	{name: "scheme with path", raw: "https://example.com/path", refused: true},
	{name: "backslash", raw: `example.com\x`, refused: true},
	{name: "user info", raw: "user@example.com", refused: true},
	{name: "query", raw: "example.com?x=1", refused: true},
	{name: "fragment", raw: "example.com#frag", refused: true},
	{name: "inner space", raw: "bad host", refused: true},
	{name: "inner tab", raw: "host\tname", refused: true},
	{name: "scheme only", raw: "http://", refused: true},
	{name: "scheme only https", raw: "https://", refused: true},
}

func TestCheckForwardHostForm(t *testing.T) {
	for _, tc := range forwardHostFormCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkForwardHostForm(tc.raw)
			if tc.refused {
				require.Error(t, err)
				assert.Equal(t, errForwardHostForm, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestCheckForwardHostForm_EmptyPassesRequiredCheckSeparately(t *testing.T) {
	assert.NoError(t, checkForwardHostForm(""))
}

// TestProxyHostService_ForwardHostFormForEveryRole runs each case through CreateAs
// for an administrator and for a non-administrator, so the form check applies to
// both roles.
func TestProxyHostService_ForwardHostFormForEveryRole(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	for _, admin := range []bool{true, false} {
		for _, tc := range forwardHostFormCases {
			t.Run(fmt.Sprintf("admin=%t/%s", admin, tc.name), func(t *testing.T) {
				host := &models.ProxyHost{
					UUID:        uuid.New().String(),
					DomainNames: "form-" + strings.ReplaceAll(uuid.New().String(), "-", "") + ".example.com",
					ForwardHost: tc.raw,
					ForwardPort: 8080,
				}
				err := service.CreateAs(host, admin)
				if tc.refused {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "forward host must be a valid IP address or hostname")
					return
				}
				// Accepted forms may still fail later checks (for example the admin listener
				// refusal); only the form error is asserted against here.
				if err != nil {
					assert.NotContains(t, err.Error(), "forward host must be a valid IP address or hostname")
				}
			})
		}
	}
}

func TestValidateLocationTargets_FormRefusal(t *testing.T) {
	for _, tc := range forwardHostFormCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLocationTargets([]models.Location{{Path: "/api", ForwardHost: tc.raw, ForwardPort: 8080}})
			if tc.refused {
				require.Error(t, err)
				assert.Equal(t, errForwardHostForm, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
