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
)

// TestBouncerRegistrationStates maps each answer from `cscli bouncers list` to one state. A failed or
// unreadable answer is unknown, never not registered; only an answer that names the bouncer's absence
// (empty, null or a list without it) is not registered.
func TestBouncerRegistrationStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output []byte
		err    error
		want   registrationState
	}{
		{name: "listed", output: []byte(`[{"name":"caddy-bouncer"}]`), want: registrationRegistered},
		{name: "other bouncer only", output: []byte(`[{"name":"other-bouncer"}]`), want: registrationNotRegistered},
		{name: "empty array", output: []byte(`[]`), want: registrationNotRegistered},
		{name: "empty output", output: []byte(``), want: registrationNotRegistered},
		{name: "null output", output: []byte(`null`), want: registrationNotRegistered},
		{name: "null with newline", output: []byte("null\n"), want: registrationNotRegistered},
		{name: "list command fails", err: errors.New("command failed"), want: registrationUnknown},
		{name: "list command times out", err: context.DeadlineExceeded, want: registrationUnknown},
		{name: "output is not JSON", output: []byte(`not valid json`), want: registrationUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &CrowdsecHandler{CmdExec: &mockCmdExecutor{output: tt.output, err: tt.err}}

			assert.Equal(t, tt.want, h.bouncerRegistration(context.Background()))
		})
	}
}

// TestGetBouncerInfoReportsUnknownWhenListFails pins the additive `registration` field: a list that
// cannot be read reports unknown, and `registered` stays false because it is only true for a confirmed
// registration.
func TestGetBouncerInfoReportsUnknownWhenListFails(t *testing.T) {
	t.Setenv("CROWDSEC_API_KEY", "test-api-key-12345678901234567890")

	h := &CrowdsecHandler{CmdExec: &mockCmdExecutor{err: errors.New("command failed")}}

	body := getBouncerInfoBody(t, h)

	assert.Equal(t, "unknown", body["registration"])
	assert.Equal(t, false, body["registered"])
	assert.Equal(t, "env_var", body["key_source"])
}

// TestGetBouncerInfoRegistrationValues checks the `registration` values reported next to `registered`.
func TestGetBouncerInfoRegistrationValues(t *testing.T) {
	t.Setenv("CROWDSEC_API_KEY", "test-api-key-12345678901234567890")

	tests := []struct {
		name           string
		output         []byte
		wantState      string
		wantRegistered bool
	}{
		{name: "registered", output: []byte(`[{"name":"caddy-bouncer"}]`), wantState: "registered", wantRegistered: true},
		{name: "not registered", output: []byte(`[{"name":"other-bouncer"}]`), wantState: "not_registered", wantRegistered: false},
		{name: "empty list", output: []byte(`null`), wantState: "not_registered", wantRegistered: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &CrowdsecHandler{CmdExec: &mockCmdExecutor{output: tt.output}}

			body := getBouncerInfoBody(t, h)

			assert.Equal(t, tt.wantState, body["registration"])
			assert.Equal(t, tt.wantRegistered, body["registered"])
		})
	}
}

// TestGetBouncerInfoMasksAKeyTooShortToPreview: a key of seven characters or fewer has no first-and-last preview,
// so only the mask is reported.
func TestGetBouncerInfoMasksAKeyTooShortToPreview(t *testing.T) {
	t.Setenv("CROWDSEC_API_KEY", "short7")

	h := &CrowdsecHandler{CmdExec: &mockCmdExecutor{output: []byte(`null`)}}

	body := getBouncerInfoBody(t, h)

	assert.Equal(t, "***", body["key_preview"])
}

// getBouncerInfoBody calls GetBouncerInfo and returns the decoded JSON body.
func getBouncerInfoBody(t *testing.T, h *CrowdsecHandler) map[string]any {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/bouncer", http.NoBody)

	h.GetBouncerInfo(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}
