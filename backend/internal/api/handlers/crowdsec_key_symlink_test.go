package handlers

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestEnsureBouncerRegistrationReplacesSymlinkedKeyWithRegularFile checks that a re-registration never writes
// through a symlinked key file: the link is replaced by a regular 0600 file and its target is left alone.
func TestEnsureBouncerRegistrationReplacesSymlinkedKeyWithRegularFile(t *testing.T) {
	for _, name := range bouncerkey.EnvNames() {
		t.Setenv(name, "")
	}
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "elsewhere")
	require.NoError(t, os.WriteFile(target, []byte("invalid-linked-key\n"), 0o600))
	keyPath := filepath.Join(tmpDir, "bouncer_key")
	require.NoError(t, os.Symlink(target, keyPath))
	t.Setenv("CHARON_CROWDSEC_BOUNCER_KEY_PATH", keyPath)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	db := setupCrowdDB(t)
	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	cfg := models.SecurityConfig{UUID: uuid.New().String(), Name: "default", CrowdSecAPIURL: server.URL}
	require.NoError(t, db.Create(&cfg).Error)

	mockCmdExec := new(MockCommandExecutor)
	mockCmdExec.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return len(args) >= 2 && args[0] == "bouncers" && args[1] == "delete"
	})).Return([]byte("deleted"), nil)
	mockCmdExec.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return len(args) >= 2 && args[0] == "bouncers" && args[1] == "add"
	})).Return([]byte("replacement-linked-key-1234567890"), nil)
	handler.CmdExec = mockCmdExec

	res, err := handler.ensureBouncerRegistration(context.Background())
	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeRegistered, res.Outcome)

	info, err := os.Lstat(keyPath)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "the link is replaced by a regular file")
	require.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
	require.Equal(t, "replacement-linked-key-1234567890", bouncerkey.ReadFile(keyPath))
	linked, err := os.ReadFile(target) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "invalid-linked-key\n", string(linked), "the link target is never written")
	mockCmdExec.AssertExpectations(t)
}
