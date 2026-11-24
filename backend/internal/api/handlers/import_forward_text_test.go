package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

func TestImportErrorTextUsesTheForwardTargetMessageForARefusal(t *testing.T) {
	t.Parallel()
	for _, refusal := range []error{services.ErrOrthrusListenerTarget, services.ErrAgentForwardTarget, services.ErrForwardTargetAddress} {
		wrapped := fmt.Errorf("check: %w", refusal)
		assert.Equal(t, forwardTargetRefusedText, importErrorText(wrapped))
	}
	assert.Equal(t, "database is locked", importErrorText(fmt.Errorf("database is locked")))
}

// A refused forward target in an import commit is reported with the same text the host form shows.
func TestImportCommitReportsARefusedForwardTargetWithTheHostFormText(t *testing.T) {
	t.Parallel()
	handler, mockImport := retentionHandler(t)
	mockImport.importResult = importableHost()
	handler.proxyHostSvc.(*mockProxyHostService).createErr = fmt.Errorf("check: %w", services.ErrForwardTargetAddress)
	path := writeUpload(t, handler.importDir, "forward.caddyfile")
	seedUploadSession(t, handler, "forward-text", path)

	w := postUploadJSON(t, handler, "/commit", map[string]any{"session_uuid": "forward-text", "resolutions": map[string]string{}})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeBody(t, w)
	errs, ok := body["errors"].([]any)
	require.True(t, ok, "errors is a list")
	require.Len(t, errs, 1)
	assert.Equal(t, "upload.example.com: "+forwardTargetRefusedText, errs[0])
}
