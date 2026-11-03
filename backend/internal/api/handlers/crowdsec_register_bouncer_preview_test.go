package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/util"
)

// registerBouncerPreview registers through the endpoint with add as the new key and returns the preview it reports.
func registerBouncerPreview(t *testing.T, add string) string {
	t.Helper()
	ex := registerCSCLI(add, nil, `[{"name":"caddy-bouncer"}]`, nil)
	h := newRegisterFixture(t, ex, map[string]bouncerkey.Verdict{regSavedKey: bouncerkey.VerdictRejected})
	writeSavedKey(t, h.bouncerKeyPath(), regSavedKey)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	preview, ok := pr4Body(t, w)["api_key_preview"].(string)
	require.True(t, ok, "api_key_preview is a string")
	return preview
}

// The helper uses t.Setenv (through newRegisterFixture), so these tests cannot run in parallel.
func TestRegisterBouncerPreviewUsesTheSharedMask(t *testing.T) {
	require.Equal(t, util.MaskAPIKey(regNewKey), registerBouncerPreview(t, regNewKey))
}

func TestRegisterBouncerPreviewIsTheMaskOfTheKeyInUse(t *testing.T) {
	require.Equal(t, "abcd...wxyz", registerBouncerPreview(t, "abcd0123456789ABCDEFGHIJKLMNwxyz"))
}
