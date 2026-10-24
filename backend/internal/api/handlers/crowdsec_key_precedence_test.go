package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/models"
)

// Keys used by the precedence tests. The LAPI stub accepts only the keys a
// test marks as valid.
const (
	precSavedKey    = "saved-key-precedence-0001"
	precEnvKey      = "env-key-precedence-00002"
	precNewKey      = "registered-key-precedence-3"
	precEnvKeyOther = "env-key-precedence-00004"
)

// keyLAPIStub is a LAPI stand-in that accepts a fixed set of keys and counts
// how often each key was presented.
type keyLAPIStub struct {
	mu       sync.Mutex
	accepted map[string]bool
	calls    map[string]int
}

func (s *keyLAPIStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("X-Api-Key")
	s.mu.Lock()
	s.calls[key]++
	ok := s.accepted[key]
	s.mu.Unlock()

	if !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"new": [], "deleted": []}`))
}

func (s *keyLAPIStub) callsFor(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[key]
}

// newKeyLAPI starts a LAPI stub that accepts the given keys.
func newKeyLAPI(t *testing.T, accepted ...string) (string, *keyLAPIStub) {
	t.Helper()
	stub := &keyLAPIStub{accepted: map[string]bool{}, calls: map[string]int{}}
	for _, k := range accepted {
		stub.accepted[k] = true
	}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	return server.URL, stub
}

// clearKeyEnv blanks every key variable, then sets the Charon name to env when
// it is not empty. Tests in this file are sequential because they use t.Setenv.
func clearKeyEnv(t *testing.T, env string) {
	t.Helper()
	for _, name := range bouncerkey.EnvNames() {
		t.Setenv(name, "")
	}
	if env != "" {
		t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", env)
	}
}

// newPrecedenceHandler builds a handler whose LAPI URL is lapiURL and whose
// key directory is dataDir. The cscli executor is the given mock.
func newPrecedenceHandler(t *testing.T, lapiURL, dataDir string, cmd CommandExecutor) *CrowdsecHandler {
	t.Helper()
	db := setupCrowdDB(t)
	cfg := models.SecurityConfig{UUID: uuid.New().String(), Name: "default", CrowdSecAPIURL: lapiURL}
	require.NoError(t, db.Create(&cfg).Error)
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	h.CmdExec = cmd
	return h
}

// bouncerRegisterMock accepts bouncer delete and add; add returns precNewKey.
func bouncerRegisterMock() *MockCommandExecutor {
	m := new(MockCommandExecutor)
	m.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return len(args) >= 2 && args[0] == "bouncers" && args[1] == "delete"
	})).Return([]byte("deleted"), nil)
	m.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return len(args) >= 2 && args[0] == "bouncers" && args[1] == "add"
	})).Return([]byte(precNewKey), nil)
	return m
}

// noCSCLIMock fails the test if any cscli command runs.
func noCSCLIMock() *MockCommandExecutor {
	return new(MockCommandExecutor)
}

func writeSavedKey(t *testing.T, path, key string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(key+"\n"), 0o600))
}

// applyCounter counts Caddy re-applies requested through handleBouncerRegistration.
type applyCounter struct{ calls int }

func (a *applyCounter) apply(_ context.Context) error {
	a.calls++
	return nil
}

func TestBouncerPrecedence_SavedValidKeyIgnoresEnvWithoutLAPICall(t *testing.T) {
	const envKey = "env-key-precedence-invalid-0005"
	lapiURL, stub := newKeyLAPI(t, precSavedKey)
	clearKeyEnv(t, envKey)
	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "bouncer_key")
	writeSavedKey(t, keyPath, precSavedKey)
	h := newPrecedenceHandler(t, lapiURL, dataDir, noCSCLIMock())
	h.envKeyRejected = true
	h.rejectedEnvKey = "stale-rejected-key"

	res, err := h.ensureBouncerRegistration(context.Background())

	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeSavedKeyValid, res.Outcome)
	require.Equal(t, precSavedKey, res.Key)
	require.True(t, res.EnvKeyIgnored)
	require.Equal(t, 0, stub.callsFor(envKey), "env key must not be sent to LAPI when a saved key is valid")
	require.Equal(t, precSavedKey, bouncerkey.ReadFile(keyPath))
	require.False(t, h.envKeyRejected, "state must clear when the saved key is valid")
	require.Empty(t, h.rejectedEnvKey)
}

func TestBouncerPrecedence_InvalidSavedKeyReplacedByValidEnvKey(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precEnvKey)
	clearKeyEnv(t, precEnvKey)
	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "bouncer_key")
	writeSavedKey(t, keyPath, "saved-key-stale-000001")
	h := newPrecedenceHandler(t, lapiURL, dataDir, noCSCLIMock())
	h.envKeyRejected = true
	h.rejectedEnvKey = "previously-rejected-key"

	res, err := h.ensureBouncerRegistration(context.Background())
	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeEnvKeySaved, res.Outcome)
	require.Equal(t, precEnvKey, bouncerkey.ReadFile(keyPath))
	require.False(t, h.envKeyRejected)
	require.Empty(t, h.rejectedEnvKey)

	apply := &applyCounter{}
	h.handleBouncerRegistration(context.Background(), res, nil, apply.apply)
	require.Equal(t, 1, apply.calls, "a saved env key needs one Caddy re-apply")
}

func TestBouncerPrecedence_BothKeysRejectedRegistersAndSetsRejectedState(t *testing.T) {
	const envKey = "env-key-precedence-rejected-6"
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, envKey)
	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "bouncer_key")
	writeSavedKey(t, keyPath, "saved-key-rejected-000007")
	h := newPrecedenceHandler(t, lapiURL, dataDir, bouncerRegisterMock())

	res, err := h.ensureBouncerRegistration(context.Background())
	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeRegistered, res.Outcome)
	require.Equal(t, precNewKey, res.Key)
	require.True(t, res.EnvKeyRejected)
	require.Equal(t, precNewKey, bouncerkey.ReadFile(keyPath))
	require.True(t, h.envKeyRejected)
	require.Equal(t, envKey, h.rejectedEnvKey)

	apply := &applyCounter{}
	h.handleBouncerRegistration(context.Background(), res, nil, apply.apply)
	require.Equal(t, 1, apply.calls, "a new registration needs one Caddy re-apply")
}

// bouncerRegisterFailMock accepts bouncer delete and fails bouncer add, so no replacement key is produced.
func bouncerRegisterFailMock() *MockCommandExecutor {
	m := new(MockCommandExecutor)
	m.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return len(args) >= 2 && args[0] == "bouncers" && args[1] == "delete"
	})).Return([]byte(""), nil)
	m.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return len(args) >= 2 && args[0] == "bouncers" && args[1] == "add"
	})).Return([]byte("cscli failed"), errors.New("exit status 1"))
	return m
}

func TestBouncerPrecedence_RejectedEnvRegisterFailureReportsNoNewKey(t *testing.T) {
	const envKey = "env-key-precedence-regfail-8"
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, envKey)
	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "bouncer_key")
	h := newPrecedenceHandler(t, lapiURL, dataDir, bouncerRegisterFailMock())

	res, err := h.ensureBouncerRegistration(context.Background())
	require.Error(t, err)
	require.True(t, res.EnvKeyRejected)
	require.True(t, h.envKeyRejected)
	require.True(t, h.rejectedEnvRegisterFailed)
	require.False(t, h.rejectedEnvSaveFailed)
	require.NoFileExists(t, keyPath, "a failed registration saves no key")

	msg := keyStatusMessage(t, h)
	require.Contains(t, msg, "could not register a new key with CrowdSec, so no new key was saved to "+keyPath)
	require.NotContains(t, msg, "saved a new key")
	require.NotContains(t, msg, "uses it")
}

func TestBouncerPrecedence_NoKeysRegisters(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	h := newPrecedenceHandler(t, lapiURL, dataDir, bouncerRegisterMock())

	res, err := h.ensureBouncerRegistration(context.Background())
	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeRegistered, res.Outcome)
	require.False(t, res.EnvKeyRejected)
	require.False(t, h.envKeyRejected)
	require.Equal(t, precNewKey, bouncerkey.ReadFile(filepath.Join(dataDir, "bouncer_key")))
}

func TestBouncerPrecedence_SaveFailureIsNotFatalForEnvKey(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precEnvKey)
	clearKeyEnv(t, precEnvKey)
	// A regular file where the key directory should be makes the save fail.
	dataDir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(dataDir, []byte("x"), 0o600))
	h := newPrecedenceHandler(t, lapiURL, dataDir, noCSCLIMock())

	res, err := h.ensureBouncerRegistration(context.Background())
	require.NoError(t, err, "a failed save must not be returned as an error")
	require.Equal(t, bouncerkey.OutcomeEnvKeySaved, res.Outcome)
	require.Equal(t, precEnvKey, res.Key)
	require.Error(t, res.SaveErr)

	apply := &applyCounter{}
	h.handleBouncerRegistration(context.Background(), res, nil, apply.apply)
	require.Equal(t, 0, apply.calls, "the file still holds the stale key, so a re-apply changes nothing")
}

func TestBouncerPrecedence_SaveFailureAfterRegistrationSkipsApply(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	dataDir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(dataDir, []byte("x"), 0o600))
	h := newPrecedenceHandler(t, lapiURL, dataDir, bouncerRegisterMock())

	res, err := h.ensureBouncerRegistration(context.Background())
	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeRegistered, res.Outcome)
	require.Equal(t, precNewKey, res.Key)
	require.Error(t, res.SaveErr)

	apply := &applyCounter{}
	h.handleBouncerRegistration(context.Background(), res, nil, apply.apply)
	require.Equal(t, 0, apply.calls, "the key file does not hold the new key, so a re-apply would not read it")
}

func TestBouncerPrecedence_RegisteredKeySavedAppliesOnce(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	h := newPrecedenceHandler(t, lapiURL, dataDir, bouncerRegisterMock())

	res, err := h.ensureBouncerRegistration(context.Background())
	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeRegistered, res.Outcome)
	require.NoError(t, res.SaveErr)
	require.Equal(t, precNewKey, bouncerkey.ReadFile(filepath.Join(dataDir, "bouncer_key")))

	apply := &applyCounter{}
	h.handleBouncerRegistration(context.Background(), res, nil, apply.apply)
	require.Equal(t, 1, apply.calls, "a registered key that was saved is applied once")
}

func TestBouncerPrecedence_RegisterFailureSkipsApply(t *testing.T) {
	apply := &applyCounter{}
	h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())

	res := bouncerkey.EnsureResult{EnvKey: precEnvKey, EnvKeyRejected: true}
	h.handleBouncerRegistration(context.Background(), res, context.DeadlineExceeded, apply.apply)
	require.Equal(t, 0, apply.calls)
}

func TestBouncerPrecedence_SavedValidEnvIgnoredLogsOnly(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precSavedKey)
	clearKeyEnv(t, precEnvKeyOther)
	h := newPrecedenceHandler(t, lapiURL, t.TempDir(), noCSCLIMock())

	apply := &applyCounter{}
	res := bouncerkey.EnsureResult{Key: precSavedKey, Outcome: bouncerkey.OutcomeSavedKeyValid, EnvKey: precEnvKeyOther, EnvKeyIgnored: true}
	h.handleBouncerRegistration(context.Background(), res, nil, apply.apply)
	require.Equal(t, 0, apply.calls, "a valid saved key never triggers a re-apply")
}

func TestBouncerPrecedence_ApplyErrorIsLoggedNotReturned(t *testing.T) {
	h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
	res := bouncerkey.EnsureResult{Key: precNewKey, Outcome: bouncerkey.OutcomeRegistered}

	called := 0
	h.handleBouncerRegistration(context.Background(), res, nil, func(context.Context) error {
		called++
		return context.Canceled
	})
	require.Equal(t, 1, called)
}

// TestBouncerPrecedence_GetKeyStatusBranches covers the four key_source branches
// of GetKeyStatus, the rejected-but-no-file case and the stateless ignored-env note.
func TestBouncerPrecedence_GetKeyStatusBranches(t *testing.T) {
	const (
		fileKey    = "file-key-status-000001"
		envKey     = "env-key-status-0000002"
		rejectedEn = "rejected-env-key-000003"
	)
	cases := []struct {
		name          string
		fileKey       string
		env           string
		rejected      bool
		saveFailed    bool
		registerFail  bool
		accepted      []string
		wantSource    string
		wantFullKey   bool
		wantValid     bool
		wantRejected  bool
		wantMsgSubstr string
	}{
		{
			name: "auto-generated: rejected env and saved file", fileKey: fileKey, env: rejectedEn, rejected: true,
			accepted: []string{fileKey}, wantSource: "auto-generated", wantFullKey: true, wantValid: true,
			wantRejected: true, wantMsgSubstr: "rejected",
		},
		{
			name: "rejected env but saved file empty falls to env without full key", fileKey: "", env: rejectedEn, rejected: true,
			wantSource: "env", wantRejected: true, wantMsgSubstr: "rejected",
		},
		{
			name: "rejected, save failed, stale file present", fileKey: fileKey, env: rejectedEn, rejected: true, saveFailed: true,
			accepted: []string{fileKey}, wantSource: "file", wantValid: true,
			wantRejected: true, wantMsgSubstr: "could not save the new key to",
		},
		{
			name: "rejected, registration failed, stale file present", fileKey: fileKey, env: rejectedEn, rejected: true, registerFail: true,
			accepted: []string{fileKey}, wantSource: "file", wantValid: true,
			wantRejected: true, wantMsgSubstr: "could not register a new key with CrowdSec",
		},
		{
			name: "file only", fileKey: fileKey, accepted: []string{fileKey},
			wantSource: "file", wantValid: true,
		},
		{
			name: "file with different env not rejected shows the ignored note", fileKey: fileKey, env: envKey,
			accepted: []string{fileKey}, wantSource: "file", wantValid: true, wantMsgSubstr: "remove the variable",
		},
		{
			name: "file with same env has no note", fileKey: fileKey, env: fileKey,
			accepted: []string{fileKey}, wantSource: "file", wantValid: true,
		},
		{
			name: "env only and accepted", env: envKey, accepted: []string{envKey},
			wantSource: "env", wantValid: true,
		},
		{
			name: "none", wantSource: "none", wantMsgSubstr: "No CrowdSec API key configured",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lapiURL, _ := newKeyLAPI(t, tc.accepted...)
			clearKeyEnv(t, tc.env)
			dataDir := t.TempDir()
			if tc.fileKey != "" {
				writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), tc.fileKey)
			}
			h := newPrecedenceHandler(t, lapiURL, dataDir, noCSCLIMock())
			h.envKeyRejected = tc.rejected
			h.rejectedEnvSaveFailed = tc.saveFailed
			h.rejectedEnvRegisterFailed = tc.registerFail
			if tc.rejected {
				h.rejectedEnvKey = rejectedEn
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/key-status", http.NoBody)
			h.GetKeyStatus(c)

			require.Equal(t, http.StatusOK, w.Code)
			var resp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.Equal(t, tc.wantSource, resp["key_source"])
			require.Equal(t, tc.wantValid, resp["valid"])
			require.Equal(t, tc.wantRejected, resp["env_key_rejected"])
			if tc.wantFullKey {
				require.Equal(t, tc.fileKey, resp["full_key"])
			} else {
				require.Empty(t, resp["full_key"])
			}
			msg, _ := resp["message"].(string)
			if tc.wantMsgSubstr == "" {
				require.Empty(t, msg)
			} else {
				require.Contains(t, msg, tc.wantMsgSubstr)
			}
		})
	}
}

func TestBouncerPrecedence_GetBouncerInfoAndKeyPreferSavedFile(t *testing.T) {
	const fileKey = "file-key-info-precedence-1"
	clearKeyEnv(t, precEnvKey)
	dataDir := t.TempDir()
	writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), fileKey)
	h := &CrowdsecHandler{
		DataDir: dataDir,
		CmdExec: &mockCmdExecutor{output: []byte(`[{"name":"caddy-bouncer"}]`)},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/bouncer", http.NoBody)
	h.GetBouncerInfo(c)
	var info map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &info))
	require.Equal(t, "file", info["key_source"])

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/bouncer/key", http.NoBody)
	h.GetBouncerKey(c)
	var key map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &key))
	require.Equal(t, fileKey, key["key"])
	require.Equal(t, "file", key["source"])
}

// TestBouncerPrecedence_GetKeyStatusRejectedMessageTracksSave checks that the rejected-key
// message claims the saved file holds the key only when the save succeeded.
func TestBouncerPrecedence_GetKeyStatusRejectedMessageTracksSave(t *testing.T) {
	const rejectedEn = "rejected-env-key-000005"

	t.Run("save succeeded", func(t *testing.T) {
		lapiURL, _ := newKeyLAPI(t, precNewKey)
		clearKeyEnv(t, rejectedEn)
		dataDir := t.TempDir()
		writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precNewKey)
		h := newPrecedenceHandler(t, lapiURL, dataDir, noCSCLIMock())
		h.envKeyRejected = true
		h.rejectedEnvKey = rejectedEn

		msg := keyStatusMessage(t, h)
		require.Contains(t, msg, "Charon saved a new key to "+h.bouncerKeyPath()+" and uses it")
	})

	t.Run("save failed", func(t *testing.T) {
		lapiURL, _ := newKeyLAPI(t)
		clearKeyEnv(t, rejectedEn)
		h := newPrecedenceHandler(t, lapiURL, t.TempDir(), noCSCLIMock())
		h.envKeyRejected = true
		h.rejectedEnvKey = rejectedEn
		h.rejectedEnvSaveFailed = true

		msg := keyStatusMessage(t, h)
		require.Contains(t, msg, "could not save the new key to "+h.bouncerKeyPath())
		require.NotContains(t, msg, "saved a new key")
		require.NotContains(t, msg, "session")
	})
}

// TestBouncerPrecedence_EnsureRecordsSaveFailureForRejectedEnv checks that a failed save after
// a rejected env key is recorded for GetKeyStatus, and that a successful save clears it.
func TestBouncerPrecedence_EnsureRecordsSaveFailureForRejectedEnv(t *testing.T) {
	t.Run("save failed", func(t *testing.T) {
		lapiURL, _ := newKeyLAPI(t)
		clearKeyEnv(t, precEnvKey)
		dataDir := filepath.Join(t.TempDir(), "not-a-directory")
		require.NoError(t, os.WriteFile(dataDir, []byte("x"), 0o600))
		h := newPrecedenceHandler(t, lapiURL, dataDir, bouncerRegisterMock())

		res, err := h.ensureBouncerRegistration(context.Background())
		require.NoError(t, err)
		require.True(t, res.EnvKeyRejected)
		require.Error(t, res.SaveErr)
		require.True(t, h.envKeyRejected)
		require.True(t, h.rejectedEnvSaveFailed)
	})

	t.Run("save succeeded", func(t *testing.T) {
		lapiURL, _ := newKeyLAPI(t)
		clearKeyEnv(t, precEnvKey)
		h := newPrecedenceHandler(t, lapiURL, t.TempDir(), bouncerRegisterMock())

		res, err := h.ensureBouncerRegistration(context.Background())
		require.NoError(t, err)
		require.NoError(t, res.SaveErr)
		require.True(t, h.envKeyRejected)
		require.False(t, h.rejectedEnvSaveFailed)
	})
}

// keyStatusMessage calls GetKeyStatus and returns the message field.
func keyStatusMessage(t *testing.T, h *CrowdsecHandler) string {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/key-status", http.NoBody)
	h.GetKeyStatus(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	msg, _ := resp["message"].(string)
	return msg
}
