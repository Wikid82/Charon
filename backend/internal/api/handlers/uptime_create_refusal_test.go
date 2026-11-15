package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/services"
)

// postMonitor sends a monitor creation request to the handler.
func postMonitor(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupUptimeCoverageDB(t)
	h := NewUptimeHandler(services.NewUptimeService(db, nil))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/uptime/monitors", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Create(c)
	return w
}

func TestUptimeCreate_RefusedDestinationIsBadRequest(t *testing.T) {
	w := postMonitor(t, `{"name":"meta","url":"tcp://169.254.169.254:80","type":"tcp"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"This destination is not allowed.","error_code":"destination_refused"}`, w.Body.String())
}

func TestUptimeCreate_InvalidInputIsBadRequestNotServerError(t *testing.T) {
	w := postMonitor(t, `{"name":"bad","url":"http://","type":"http"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "URL must include host")
}

func TestUptimeCreate_AllowedDestinationIsCreated(t *testing.T) {
	w := postMonitor(t, `{"name":"lan","url":"tcp://192.168.1.1:5432","type":"tcp"}`)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}
