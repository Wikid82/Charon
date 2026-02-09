package handlers

import (
	"bytes"
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

// mockCommandExecutor is a mock implementation of CommandExecutor for testing
type mockCommandExecutor struct {
	output []byte
	err    error
	calls  [][]string // Track all calls made
}

func (m *mockCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	return m.output, m.err
}

func TestListDecisions_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		output: []byte(`[{"id":1,"origin":"cscli","type":"ban","scope":"ip","value":"192.168.1.100","duration":"4h","scenario":"manual 'ban' from 'localhost'","created_at":"2025-12-05T10:00:00Z","until":"2025-12-05T14:00:00Z"}]`),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	decisions := resp["decisions"].([]any)
	assert.Len(t, decisions, 1)

	decision := decisions[0].(map[string]any)
	assert.Equal(t, "192.168.1.100", decision["value"])
	assert.Equal(t, "ban", decision["type"])
	assert.Equal(t, "ip", decision["scope"])

	// Verify cscli was called with correct args
	require.Len(t, mockExec.calls, 1)
	assert.Equal(t, []string{"cscli", "decisions", "list", "-o", "json"}, mockExec.calls[0])
}

func TestListDecisions_EmptyList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		output: []byte("null"),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	decisions := resp["decisions"].([]any)
	assert.Len(t, decisions, 0)
	assert.Equal(t, float64(0), resp["total"])
}

func TestListDecisions_CscliError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		err: errors.New("cscli not found"),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions", http.NoBody)
	r.ServeHTTP(w, req)

	// Should return 200 with empty list and error message
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	decisions := resp["decisions"].([]any)
	assert.Len(t, decisions, 0)
	assert.Contains(t, resp["error"], "cscli not available")
}

func TestListDecisions_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		output: []byte("invalid json"),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to parse decisions")
}

func TestBanIP_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		output: []byte(""),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	payload := BanIPRequest{
		IP:       "192.168.1.100",
		Duration: "24h",
		Reason:   "suspicious activity",
	}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/ban", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "banned", resp["status"])
	assert.Equal(t, "192.168.1.100", resp["ip"])
	assert.Equal(t, "24h", resp["duration"])

	// Verify cscli was called with correct args
	require.Len(t, mockExec.calls, 1)
	assert.Equal(t, "cscli", mockExec.calls[0][0])
	assert.Equal(t, "decisions", mockExec.calls[0][1])
	assert.Equal(t, "add", mockExec.calls[0][2])
	assert.Equal(t, "-i", mockExec.calls[0][3])
	assert.Equal(t, "192.168.1.100", mockExec.calls[0][4])
	assert.Equal(t, "-d", mockExec.calls[0][5])
	assert.Equal(t, "24h", mockExec.calls[0][6])
	assert.Equal(t, "-R", mockExec.calls[0][7])
	assert.Equal(t, "manual ban: suspicious activity", mockExec.calls[0][8])
}

func TestBanIP_DefaultDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		output: []byte(""),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	payload := BanIPRequest{
		IP: "10.0.0.1",
	}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/ban", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Duration should default to 24h
	assert.Equal(t, "24h", resp["duration"])

	// Verify cscli was called with default duration
	require.Len(t, mockExec.calls, 1)
	assert.Equal(t, "24h", mockExec.calls[0][6])
}

func TestBanIP_MissingIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	payload := map[string]string{}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/ban", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ip is required")
}

func TestBanIP_EmptyIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	payload := BanIPRequest{
		IP: "   ",
	}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/ban", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ip cannot be empty")
}

func TestBanIP_CscliError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		err: errors.New("cscli failed"),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	payload := BanIPRequest{
		IP: "192.168.1.100",
	}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/ban", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to ban IP")
}

func TestUnbanIP_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		output: []byte(""),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/ban/192.168.1.100", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "unbanned", resp["status"])
	assert.Equal(t, "192.168.1.100", resp["ip"])

	// Verify cscli was called with correct args
	require.Len(t, mockExec.calls, 1)
	assert.Equal(t, []string{"cscli", "decisions", "delete", "-i", "192.168.1.100"}, mockExec.calls[0])
}

func TestUnbanIP_CscliError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		err: errors.New("cscli failed"),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/ban/192.168.1.100", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to unban IP")
}

func TestListDecisions_MultipleDecisions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	mockExec := &mockCommandExecutor{
		output: []byte(`[
			{"id":1,"origin":"cscli","type":"ban","scope":"ip","value":"192.168.1.100","duration":"4h","scenario":"manual ban","created_at":"2025-12-05T10:00:00Z"},
			{"id":2,"origin":"crowdsec","type":"ban","scope":"ip","value":"10.0.0.50","duration":"1h","scenario":"ssh-bf","created_at":"2025-12-05T11:00:00Z"},
			{"id":3,"origin":"cscli","type":"ban","scope":"range","value":"172.16.0.0/24","duration":"24h","scenario":"manual ban","created_at":"2025-12-05T12:00:00Z"}
		]`),
	}

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.CmdExec = mockExec

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	decisions := resp["decisions"].([]any)
	assert.Len(t, decisions, 3)
	assert.Equal(t, float64(3), resp["total"])

	// Verify each decision
	d1 := decisions[0].(map[string]any)
	assert.Equal(t, "192.168.1.100", d1["value"])
	assert.Equal(t, "cscli", d1["origin"])

	d2 := decisions[1].(map[string]any)
	assert.Equal(t, "10.0.0.50", d2["value"])
	assert.Equal(t, "crowdsec", d2["origin"])
	assert.Equal(t, "ssh-bf", d2["scenario"])

	d3 := decisions[2].(map[string]any)
	assert.Equal(t, "172.16.0.0/24", d3["value"])
	assert.Equal(t, "range", d3["scope"])
}

func TestBanIP_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/ban", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ip is required")
}
