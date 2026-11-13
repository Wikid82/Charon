package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
)

// recordingOriginRefresher counts how often the settings API asks the browser origin guard to refresh.
type recordingOriginRefresher struct {
	calls int
	err   error
}

func (r *recordingOriginRefresher) Refresh() error {
	r.calls++
	return r.err
}

func postJSON(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestUpdateSetting_PublicURLRefreshesBrowserOrigin(t *testing.T) {
	db := setupSettingsTestDB(t)
	refresher := &recordingOriginRefresher{}
	handler := handlers.NewSettingsHandler(db).WithBrowserOriginRefresher(refresher)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	w := postJSON(t, router, http.MethodPost, "/settings", `{"key":"app.public_url","value":"https://charon.example.com"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, refresher.calls)
}

func TestUpdateSetting_OtherKeyDoesNotRefreshBrowserOrigin(t *testing.T) {
	db := setupSettingsTestDB(t)
	refresher := &recordingOriginRefresher{}
	handler := handlers.NewSettingsHandler(db).WithBrowserOriginRefresher(refresher)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	w := postJSON(t, router, http.MethodPost, "/settings", `{"key":"general.site_title","value":"Charon"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, refresher.calls)
}

func TestUpdateSetting_PublicURLRefreshFailureKeepsSavedSetting(t *testing.T) {
	db := setupSettingsTestDB(t)
	refresher := &recordingOriginRefresher{err: errors.New("bad origin")}
	handler := handlers.NewSettingsHandler(db).WithBrowserOriginRefresher(refresher)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	w := postJSON(t, router, http.MethodPost, "/settings", `{"key":"app.public_url","value":"https://charon.example.com"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, refresher.calls)
	var stored models.Setting
	require.NoError(t, db.Where("key = ?", "app.public_url").First(&stored).Error)
	assert.Equal(t, "https://charon.example.com", stored.Value)
}

func TestPatchConfig_PublicURLRefreshesBrowserOrigin(t *testing.T) {
	db := setupSettingsTestDB(t)
	refresher := &recordingOriginRefresher{}
	handler := handlers.NewSettingsHandler(db).WithBrowserOriginRefresher(refresher)
	router := newAdminRouter()
	router.PATCH("/config", handler.PatchConfig)

	w := postJSON(t, router, http.MethodPatch, "/config", `{"app":{"public_url":"https://charon.example.com"}}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, refresher.calls)

	w = postJSON(t, router, http.MethodPatch, "/config", `{"general":{"site_title":"Charon"}}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, refresher.calls, "an unrelated key must not refresh the origin guard")
}

func TestUpdateSetting_PublicURLWithoutRefresherIsSaved(t *testing.T) {
	db := setupSettingsTestDB(t)
	handler := handlers.NewSettingsHandler(db) // no origin guard wired
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	w := postJSON(t, router, http.MethodPost, "/settings", `{"key":"app.public_url","value":"https://charon.example.com"}`)

	require.Equal(t, http.StatusOK, w.Code)
	var stored models.Setting
	require.NoError(t, db.Where("key = ?", "app.public_url").First(&stored).Error)
	assert.Equal(t, "https://charon.example.com", stored.Value)
}
