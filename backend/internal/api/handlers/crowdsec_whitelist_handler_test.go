package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// reloadRecorder installs a fake managed CrowdSec process and records SIGHUP delivery.
type reloadRecorder struct {
	signalled []int
	sigs      []syscall.Signal
	signalErr error
}

func (r *reloadRecorder) attach(h *CrowdsecHandler, running bool) {
	h.Executor = &fakeExec{started: running}
	h.signalProcess = func(pid int, sig syscall.Signal) error {
		r.signalled = append(r.signalled, pid)
		r.sigs = append(r.sigs, sig)
		return r.signalErr
	}
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/whitelist", http.NoBody)
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
	mock := &reloadRecorder{}
	mock.attach(h, true)

	body := `{"ip_or_cidr":"1.2.3.4","reason":"test"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, []int{12345}, mock.signalled)
	assert.Equal(t, []syscall.Signal{syscall.SIGHUP}, mock.sigs)

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
	mock := &reloadRecorder{}
	mock.attach(h, true)

	svc := services.NewCrowdSecWhitelistService(db, "")
	entry, err := svc.Add(t.Context(), "7.7.7.7", "to delete")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/"+entry.UUID, http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, []int{12345}, mock.signalled)
	assert.Equal(t, []syscall.Signal{syscall.SIGHUP}, mock.sigs)
}

func TestDeleteWhitelist_NotFound(t *testing.T) {
	t.Parallel()
	_, r, _ := setupWhitelistHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/00000000-0000-0000-0000-000000000000", http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/whitelist", http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/whitelist", http.NoBody)
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
	mock := &reloadRecorder{signalErr: errors.New("signal failed")}
	mock.attach(h, true)

	body := `{"ip_or_cidr":"3.3.3.3","reason":"reload test"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, []int{12345}, mock.signalled)
	assert.Equal(t, []syscall.Signal{syscall.SIGHUP}, mock.sigs)
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
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/"+entry.UUID, http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "failed to delete whitelist entry", resp["error"])
}

func TestDeleteWhitelist_ReloadFailure(t *testing.T) {
	t.Parallel()
	h, r, db := setupWhitelistHandler(t)
	mock := &reloadRecorder{signalErr: errors.New("signal failed")}
	mock.attach(h, true)

	svc := services.NewCrowdSecWhitelistService(db, "")
	entry, err := svc.Add(t.Context(), "5.5.5.5", "reload test")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/"+entry.UUID, http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, []int{12345}, mock.signalled)
	assert.Equal(t, []syscall.Signal{syscall.SIGHUP}, mock.sigs)
}

func TestDeleteWhitelist_EmptyUUID(t *testing.T) {
	t.Parallel()
	h, _, _ := setupWhitelistHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/whitelist/", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: ""}}

	h.DeleteWhitelist(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "uuid is required", resp["error"])
}

func TestAddWhitelist_CrowdSecNotRunningSkipsSignal(t *testing.T) {
	t.Parallel()
	h, r, _ := setupWhitelistHandler(t)
	mock := &reloadRecorder{}
	mock.attach(h, false)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/whitelist", bytes.NewBufferString(`{"ip_or_cidr":"6.6.6.6"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Empty(t, mock.signalled)
}

func TestReloadCrowdSec(t *testing.T) {
	t.Parallel()
	h, _, _ := setupWhitelistHandler(t)
	mock := &reloadRecorder{}
	mock.attach(h, true)
	require.NoError(t, h.Hub.Reload(t.Context()), "hub service must reload through the handler")
	assert.Equal(t, []syscall.Signal{syscall.SIGHUP}, mock.sigs)

	mock = &reloadRecorder{}
	mock.attach(h, false)
	require.ErrorIs(t, h.reloadCrowdSec(t.Context()), crowdsec.ErrCrowdSecNotRunning)

	mock = &reloadRecorder{signalErr: errors.New("boom")}
	mock.attach(h, true)
	require.Error(t, h.reloadCrowdSec(t.Context()))

	h.Executor = nil
	require.ErrorIs(t, h.reloadCrowdSec(t.Context()), crowdsec.ErrCrowdSecNotRunning)

	h.signalProcess = nil
	h.Executor = &fakeExec{started: false}
	require.ErrorIs(t, h.reloadCrowdSec(t.Context()), crowdsec.ErrCrowdSecNotRunning)
	require.Error(t, signalOSProcess(-1, syscall.SIGHUP))
}
