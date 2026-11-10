package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/alerts"
	"github.com/Wikid82/charon/backend/internal/models"
)

func serveAlerts(t *testing.T, svc *alerts.Service) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/notifications/alerts", NewAlertsHandler(svc).List)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/notifications/alerts", http.NoBody))
	return w
}

func TestAlertsHandler_EmptyStateIsEmptyArray(t *testing.T) {
	w := serveAlerts(t, alerts.NewService())

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"alerts": []}`, w.Body.String())
}

func TestAlertsHandler_ReturnsAlertContract(t *testing.T) {
	available := false
	svc := alerts.NewService(func(context.Context) (alerts.Alert, bool, error) {
		return alerts.Alert{
			Key:                alerts.KeyGeoIPUnavailable,
			Severity:           models.NotificationTypeError,
			AffectedLists:      []string{"Office only", "No RU/CN"},
			AffectedListsTotal: 2,
			DatabaseAvailable:  &available,
		}, true, nil
	})

	w := serveAlerts(t, svc)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"alerts": [{
		"key": "geoip_unavailable",
		"severity": "error",
		"affected_lists": ["No RU/CN", "Office only"],
		"affected_lists_total": 2,
		"database_available": false
	}]}`, w.Body.String())
}

func TestAlertsHandler_FailingSourceStillReturns200(t *testing.T) {
	svc := alerts.NewService(func(context.Context) (alerts.Alert, bool, error) {
		return alerts.Alert{}, false, errors.New("source broke")
	})

	w := serveAlerts(t, svc)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Alerts []json.RawMessage `json:"alerts"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotNil(t, body.Alerts)
	assert.Empty(t, body.Alerts)
}
