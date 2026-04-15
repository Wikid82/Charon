package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockCmdExecWhitelist struct {
	reloadCalled bool
	reloadErr    error
}

func (m *mockCmdExecWhitelist) Execute(_ context.Context, _ string, _ ...string) ([]byte, error) {
	m.reloadCalled = true
	return nil, m.reloadErr
}

func setupWhitelistHandler(t *testing.T) (*CrowdsecHandler, *gin.Engine, *gorm.DB) {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdSecWhitelist{}))
	fe := &fakeExec{}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", "")
	h.WhitelistSvc = services.NewCrowdSecWhitelistService(db, "")

	r := gin.New()
	g := r.Group("/api/v1")
	g.GET("/admin/crowdsec/whitelist", h.ListWhitelists)
	g.POST("/admin/crowdsec/whitelist", h.AddWhitelist)
	g.DELETE("/admin/crowdsec/whitelist/:uuid", h.DeleteWhitelist)

	return h, r, db
}

func TestListWhitelists_Empty(t *testing.T) {
	t.Parallel()
	_, r, _ := setupWhitelistHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/whitelist", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	entries, ok := resp["entries"].([]interface{})
	assert.True(t, ok)
	assert.Empty(t, entries)
}

func TestAddWhitelist_ValidIP(t *testing.T) {
	t.Parallel()
	h, r, _ := setupWhitelistHandler(t)
	mock := &mockCmdExecWhitelist{}
	h.CmdExec = mock

	body := `{"ip_or_cidr":"1.2.3.4","reason":"test"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, mock.reloadCalled)

	var entry models.CrowdSecWhitelist
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.Equal(t, "1.2.3.4", entry.IPOrCIDR)
	assert.NotEmpty(t, entry.UUID)
}

func TestAddWhitelist_InvalidIP(t *testing.T) {
	t.Parallel()
	_, r, _ := setupWhitelistHandler(t)

	body := `{"ip_or_cidr":"not-valid","reason":""}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddWhitelist_Duplicate(t *testing.T) {
	t.Parallel()
	_, r, _ := setupWhitelistHandler(t)

	body := `{"ip_or_cidr":"9.9.9.9","reason":""}`
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i == 0 {
			assert.Equal(t, http.StatusCreated, w.Code)
		} else {
			assert.Equal(t, http.StatusConflict, w.Code)
		}
	}
}

func TestDeleteWhitelist_Existing(t *testing.T) {
	t.Parallel()
	h, r, db := setupWhitelistHandler(t)
	mock := &mockCmdExecWhitelist{}
	h.CmdExec = mock

	svc := services.NewCrowdSecWhitelistService(db, "")
	entry, err := svc.Add(t.Context(), "7.7.7.7", "to delete")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/"+entry.UUID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, mock.reloadCalled)
}

func TestDeleteWhitelist_NotFound(t *testing.T) {
	t.Parallel()
	_, r, _ := setupWhitelistHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/00000000-0000-0000-0000-000000000000", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListWhitelists_AfterAdd(t *testing.T) {
	t.Parallel()
	_, r, db := setupWhitelistHandler(t)
	svc := services.NewCrowdSecWhitelistService(db, "")
	_, err := svc.Add(t.Context(), "8.8.8.8", "google dns")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/whitelist", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	entries := resp["entries"].([]interface{})
	assert.Len(t, entries, 1)
}
