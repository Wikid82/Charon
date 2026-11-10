package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Formats that embed the private key must ask for the admin password even
// when include_key is false or omitted.
func keyFormatExport(format string, includeKey *bool, password string) map[string]any {
	body := map[string]any{"format": format, "pfx_password": "pfx-passphrase"}
	if includeKey != nil {
		body["include_key"] = *includeKey
	}
	if password != "" {
		body["password"] = password
	}
	return body
}

func boolPtr(v bool) *bool { return &v }

func TestCertificateHandler_Export_KeyFormatRequiresPassword(t *testing.T) {
	cases := []struct {
		name       string
		format     string
		includeKey *bool
	}{
		{name: "pfx include_key omitted", format: "pfx"},
		{name: "pfx include_key false", format: "pfx", includeKey: boolPtr(false)},
		{name: "p12 include_key omitted", format: "p12"},
		{name: "p12 include_key false", format: "p12", includeKey: boolPtr(false)},
		{name: "upper-case PFX include_key omitted", format: "PFX"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			guard := &fakePasswordGuard{allow: true}
			e := newExportHarness(t, guard)
			_, token := e.createUser(models.RoleAdmin)

			w := e.export(token, keyFormatExport(tc.format, tc.includeKey, ""))

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "password required")
			assert.Zero(t, guard.calls, "a request with no password must not consume login budget")
			assert.NotEqual(t, "application/octet-stream", w.Header().Get("Content-Type"))
		})
	}
}

func TestCertificateHandler_Export_KeyFormatWrongPasswordRefused(t *testing.T) {
	for _, format := range []string{"pfx", "p12"} {
		t.Run(format, func(t *testing.T) {
			e := newExportHarness(t, &fakePasswordGuard{allow: true})
			_, token := e.createUser(models.RoleAdmin)

			w := e.export(token, keyFormatExport(format, nil, "wrong-password"))

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "incorrect password")
		})
	}
}

func TestCertificateHandler_Export_KeyFormatWithPasswordSucceeds(t *testing.T) {
	for _, format := range []string{"pfx", "p12"} {
		t.Run(format, func(t *testing.T) {
			e := newExportHarness(t, &fakePasswordGuard{allow: true})
			_, token := e.createUser(models.RoleAdmin)

			w := e.export(token, keyFormatExport(format, nil, exportTestPassword))

			require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
			assert.Contains(t, w.Header().Get("Content-Disposition"), "export-cert.pfx")
			assert.NotEmpty(t, w.Body.Bytes())
		})
	}
}

func TestCertificateHandler_Export_KeyFormatEmergencyBypassRefused(t *testing.T) {
	for _, format := range []string{"pfx", "p12"} {
		t.Run(format, func(t *testing.T) {
			guard := &fakePasswordGuard{allow: true}
			e := newExportHarness(t, guard, bypassMiddleware())

			w := e.export("", keyFormatExport(format, nil, exportTestPassword))

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "authenticated user session")
			assert.Zero(t, guard.calls, "bypass sessions never reach password verification")
		})
	}
}

func TestCertificateHandler_Export_PEMWithoutKeyStillNoPassword(t *testing.T) {
	e := newExportHarness(t, &fakePasswordGuard{allow: true})
	_, token := e.createUser(models.RoleAdmin)

	w := e.export(token, keyFormatExport("der", nil, ""))

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}
