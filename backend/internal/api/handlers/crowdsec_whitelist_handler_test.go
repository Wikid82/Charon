package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	entries, ok := resp["whitelist"].([]interface{})
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
	entries := resp["whitelist"].([]interface{})
	assert.Len(t, entries, 1)
}

func TestAddWhitelist_400_MissingField(t *testing.T) {
	t.Parallel()
	_, r, _ := setupWhitelistHandler(t)

	body := `{}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ip_or_cidr is required", resp["error"])
}

func TestListWhitelists_DBError(t *testing.T) {
	t.Parallel()
	_, r, db := setupWhitelistHandler(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/whitelist", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "failed to list whitelist entries", resp["error"])
}

func TestAddWhitelist_DBError(t *testing.T) {
	t.Parallel()
	_, r, db := setupWhitelistHandler(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	body := `{"ip_or_cidr":"1.2.3.4","reason":"test"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "failed to add whitelist entry", resp["error"])
}

func TestAddWhitelist_ReloadFailure(t *testing.T) {
	t.Parallel()
	h, r, _ := setupWhitelistHandler(t)
	mock := &mockCmdExecWhitelist{reloadErr: errors.New("cscli failed")}
	h.CmdExec = mock

	body := `{"ip_or_cidr":"3.3.3.3","reason":"reload test"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, mock.reloadCalled)
}

func TestDeleteWhitelist_DBError(t *testing.T) {
	t.Parallel()
	_, r, db := setupWhitelistHandler(t)
	svc := services.NewCrowdSecWhitelistService(db, "")
	entry, err := svc.Add(t.Context(), "4.4.4.4", "will close db")
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/"+entry.UUID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "failed to delete whitelist entry", resp["error"])
}

func TestDeleteWhitelist_ReloadFailure(t *testing.T) {
	t.Parallel()
	h, r, db := setupWhitelistHandler(t)
	mock := &mockCmdExecWhitelist{reloadErr: errors.New("cscli failed")}
	h.CmdExec = mock

	svc := services.NewCrowdSecWhitelistService(db, "")
	entry, err := svc.Add(t.Context(), "5.5.5.5", "reload test")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/"+entry.UUID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, mock.reloadCalled)
}

func TestDeleteWhitelist_EmptyUUID(t *testing.T) {
	t.Parallel()
	h, _, _ := setupWhitelistHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/", nil)
	c.Params = gin.Params{{Key: "uuid", Value: ""}}

	h.DeleteWhitelist(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "uuid is required", resp["error"])
}
