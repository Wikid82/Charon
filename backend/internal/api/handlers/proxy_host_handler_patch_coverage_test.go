package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateForwardHostWarnings_PrivateIP(t *testing.T) {
	warnings := generateForwardHostWarnings("192.168.1.100")
	require.Len(t, warnings, 1)
	assert.Equal(t, "forward_host", warnings[0].Field)
}

func TestBulkUpdateSecurityHeaders_AllFail_Rollback(t *testing.T) {
	r, _ := setupTestRouterForSecurityHeaders(t)

	body, err := json.Marshal(map[string]any{
		"host_uuids": []string{
			uuid.New().String(),
			uuid.New().String(),
			uuid.New().String(),
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkUpdateSecurityHeaders_ProfileDB_NonNotFoundError(t *testing.T) {
	r, db := setupTestRouterForSecurityHeaders(t)

	// Drop the security_header_profiles table so the lookup returns a non-NotFound DB error
	require.NoError(t, db.Exec("DROP TABLE security_header_profiles").Error)

	profileID := uint(1)
	body, err := json.Marshal(map[string]any{
		"host_uuids":                 []string{uuid.New().String()},
		"security_header_profile_id": profileID,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGenerateForwardHostWarnings_DockerBridgeIP(t *testing.T) {
	warnings := generateForwardHostWarnings("172.17.0.1")
	require.Len(t, warnings, 1)
	assert.Equal(t, "forward_host", warnings[0].Field)
}

func TestParseNullableUintField_DefaultType(t *testing.T) {
	id, exists, err := parseNullableUintField(true, "test_field")
	assert.Nil(t, id)
	assert.True(t, exists)
	assert.Error(t, err)
}

func TestParseForwardPortField_StringEmpty(t *testing.T) {
	_, err := parseForwardPortField("")
	assert.Error(t, err)
}

func TestParseForwardPortField_StringNonNumeric(t *testing.T) {
	_, err := parseForwardPortField("notaport")
	assert.Error(t, err)
}

func TestParseForwardPortField_StringValid(t *testing.T) {
	port, err := parseForwardPortField("8080")
	require.NoError(t, err)
	assert.Equal(t, 8080, port)
}

func TestParseForwardPortField_DefaultType(t *testing.T) {
	_, err := parseForwardPortField(true)
	assert.Error(t, err)
}

func TestCreate_InvalidCertificateRef(t *testing.T) {
	r, _ := setupTestRouterForSecurityHeaders(t)

	body, err := json.Marshal(map[string]any{
		"domain_names":   "cert-ref.example.com",
		"forward_host":   "localhost",
		"forward_port":   8080,
		"certificate_id": uuid.New().String(),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreate_InvalidSecurityHeaderProfileRef(t *testing.T) {
	r, _ := setupTestRouterForSecurityHeaders(t)

	body, err := json.Marshal(map[string]any{
		"domain_names":               "shp-ref.example.com",
		"forward_host":               "localhost",
		"forward_port":               8080,
		"security_header_profile_id": uuid.New().String(),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
