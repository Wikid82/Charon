package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

type referenceViolationError struct{}

func (referenceViolationError) Error() string { return "FOREIGN KEY constraint failed" }

func TestBulkFailureText(t *testing.T) {
	assert.Equal(t, "a selected item was deleted", bulkFailureText(referenceViolationError{}))
	assert.Equal(t, "disk full", bulkFailureText(errors.New("disk full")))
}

func TestRespondHostWriteError_ReferenceViolationIsConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", http.NoBody)

	respondHostWriteError(c, fmt.Errorf("save host: %w", referenceViolationError{}))

	assert.Equal(t, http.StatusConflict, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "reference_missing", body["error_code"])
}

func TestProxyHostHandler_BulkUpdateACL_MissingAccessList(t *testing.T) {
	t.Parallel()
	router, db := setupTestRouter(t)

	host := &models.ProxyHost{
		UUID: uuid.NewString(), Name: "Host", DomainNames: "missing-acl.example.com",
		ForwardScheme: "http", ForwardHost: "localhost", ForwardPort: 8010, Enabled: true,
	}
	require.NoError(t, db.Create(host).Error)

	body := fmt.Sprintf(`{"host_uuids":["%s"],"access_list_id":9999}`, host.UUID)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "access list not found")

	var stored models.ProxyHost
	require.NoError(t, db.First(&stored, "uuid = ?", host.UUID).Error)
	assert.Nil(t, stored.AccessListID, "no host may change when the list is missing")
}
