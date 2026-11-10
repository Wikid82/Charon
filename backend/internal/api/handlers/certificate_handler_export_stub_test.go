package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// stubCertificateExporter stands in for the certificate service during export, returning a fixed error.
type stubCertificateExporter struct {
	err error
}

func (s stubCertificateExporter) ExportCertificate(_, _ string, _ services.ExportOptions) (data []byte, filename string, err error) {
	return nil, "", s.err
}

// TestCertificateHandler_Export_NotAuthorizedFromServiceIsForbidden covers the branch that maps the service's
// authorisation refusal to 403. The handler passes the key only after re-authentication, so the service
// refusal is reached only when the exporter reports it.
func TestCertificateHandler_Export_NotAuthorizedFromServiceIsForbidden(t *testing.T) {
	e := newExportHarness(t, nil)
	_, token := e.createUser(models.RoleAdmin)
	e.handler.exporter = stubCertificateExporter{err: services.ErrKeyExportNotAuthorized}

	w := e.export(token, keyExport(exportTestPassword))

	require.Equal(t, http.StatusForbidden, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "private key export requires re-authentication")
}
