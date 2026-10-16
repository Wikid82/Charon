package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func registerBouncerPreview(t *testing.T, scriptOutput string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "register_bouncer.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n"), 0o600))
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.registerScriptPath = script
	h.CmdExec = &mockCmdExecutor{output: []byte(scriptOutput)}
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/bouncer/register", http.NoBody))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Preview string `json:"api_key_preview"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Preview
}

func TestRegisterBouncerPreviewUsesTheSharedMask(t *testing.T) {
	t.Parallel()

	preview := registerBouncerPreview(t, "Bouncer registered successfully\nabcd0123456789ABCDEFGHIJKLMNwxyz\n")

	require.Equal(t, "abcd...wxyz", preview)
}

func TestRegisterBouncerPreviewIsEmptyWithoutAKeyLine(t *testing.T) {
	t.Parallel()

	require.Empty(t, registerBouncerPreview(t, "Bouncer registered successfully\n"))
}
