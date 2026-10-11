package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

const diagnosticsSentinel = "ZZsentinel0123456789abcdefghijklmnopqrstuvwx"

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	logger.Init(false, buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })
	return buf
}

func TestRegisterAndSaveBouncer_ErrorOmitsOutput(t *testing.T) {
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.CmdExec = &mockCmdExecutor{output: []byte(diagnosticsSentinel + " boom"), err: errors.New("exit status 1")}

	_, err := h.registerAndSaveBouncer(context.Background())

	require.Error(t, err)
	require.NotContains(t, err.Error(), diagnosticsSentinel)
	require.Contains(t, err.Error(), "bytes of output omitted")
	require.ErrorContains(t, err, "exit status 1")
}

func TestRegisterBouncer_FailureDetailsFixed(t *testing.T) {
	buf := captureLogs(t)
	script := filepath.Join(t.TempDir(), "register_bouncer.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n"), 0o600))

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.registerScriptPath = script
	h.CmdExec = &mockCmdExecutor{output: []byte(diagnosticsSentinel + " boom"), err: errors.New("exit status 1")}

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/bouncer/register", http.NoBody))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), `"details":"see server logs"`)
	require.NotContains(t, w.Body.String(), diagnosticsSentinel)
	require.NotContains(t, buf.String(), diagnosticsSentinel)
	require.Contains(t, buf.String(), "bytes of output omitted")
}

func TestBouncerKeyEndpoints_NonAdminForbidden(t *testing.T) {
	t.Parallel()

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	admin := r.Group("/api/v1")
	admin.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, string(models.RoleUser))
		c.Next()
	}, middleware.RequireRole(models.RoleAdmin))
	h.RegisterRoutes(admin)

	for _, path := range []string{"/api/v1/admin/crowdsec/bouncer/key", "/api/v1/admin/crowdsec/key-status"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		require.Equal(t, http.StatusForbidden, w.Code, path)
	}
}
