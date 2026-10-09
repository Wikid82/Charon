package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestRecordPresetEventSuccessDefaults(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}))
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())

	h.recordPresetEvent("req-slug", crowdsec.ApplyResult{CacheKey: "k", BackupPath: "/b"}, nil)
	h.recordPresetEvent("req-slug", crowdsec.ApplyResult{Status: "applied", AppliedPreset: "resolved"}, nil)

	var events []models.CrowdsecPresetEvent
	require.NoError(t, db.Order("id").Find(&events).Error)
	require.Len(t, events, 2)
	require.Equal(t, "req-slug", events[0].Slug)
	require.Equal(t, "applied", events[0].Status)
	require.Equal(t, "k", events[0].CacheKey)
	require.Equal(t, "/b", events[0].BackupPath)
	require.Equal(t, "resolved", events[1].Slug)
}

func TestRecordPresetEventFailure(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}))
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())

	h.recordPresetEvent("s", crowdsec.ApplyResult{AppliedPreset: "other", Status: "applied"}, errors.New("boom"))

	var events []models.CrowdsecPresetEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "s", events[0].Slug)
	require.Equal(t, "failed", events[0].Status)
	require.Equal(t, "boom", events[0].Error)
}

func TestRecordPresetEventDBErrorDoesNotPanic(t *testing.T) {
	// No CrowdsecPresetEvent table: Create fails and must only be logged.
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	require.NotPanics(t, func() {
		h.recordPresetEvent("s", crowdsec.ApplyResult{Status: "applied"}, nil)
		h.recordPresetEvent("s", crowdsec.ApplyResult{}, errors.New("x"))
	})

	h.DB = nil
	require.NotPanics(t, func() { h.recordPresetEvent("s", crowdsec.ApplyResult{}, nil) })
}

func TestApplyResponseHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondApplySuccess(c, crowdsec.ApplyResult{Status: "applied", BackupPath: "/b", ReloadHint: true, UsedCSCLI: true, CacheKey: "k", AppliedPreset: "s"})
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"status":"applied","backup":"/b","reload_hint":true,"used_cscli":true,"cache_key":"k","slug":"s"}`, w.Body.String())

	require.Equal(t, gin.H{"error": "m"}, applyFailureBody("m", crowdsec.ApplyResult{}))
	require.Equal(t, gin.H{"error": "m", "backup": "/b", "cache_key": "k"}, applyFailureBody("m", crowdsec.ApplyResult{BackupPath: "/b", CacheKey: "k"}))
}
