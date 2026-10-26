package handlers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// A handler without a whitelist service has no file to write, so the write reports nothing written and no error.
func TestWriteWhitelistFile_HandlerWithoutServiceWritesNothing(t *testing.T) {
	h := &CrowdsecHandler{}

	written, err := h.writeWhitelistFile(context.Background())

	require.NoError(t, err)
	assert.False(t, written)
}

// A sync whose data-lock wait ends with its context returns at once and leaves the whitelist file alone.
func TestSyncWhitelistFile_CancelledLockWaitSkipsRewrite(t *testing.T) {
	db := setupCrowdDB(t)
	dataDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	require.NotNil(t, h.WhitelistSvc, "the handler must own a whitelist service so a completed sync would write")

	// Hold the data lock so the sync cannot take it, then cancel the wait before it starts.
	require.NoError(t, h.dataMu.LockContext(context.Background()))
	defer h.dataMu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h.syncWhitelistFile(ctx, false)

	_, statErr := os.Stat(filepath.Join(dataDir, filepath.FromSlash(crowdsec.WhitelistRelPath)))
	assert.True(t, os.IsNotExist(statErr), "the whitelist file must not be rewritten without the data lock")
}
