package bouncerkey

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// clearEnv blanks every key variable so each test starts from a known environment.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, name := range EnvNames() {
		t.Setenv(name, "")
	}
}

func TestDefaultFile(t *testing.T) {
	require.Equal(t, "/app/data/crowdsec/bouncer_key", DefaultFile)
}

func TestEnvNamesOrder(t *testing.T) {
	require.Equal(t, []string{
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}, EnvNames())
}

func TestEnvNamesReturnsCopy(t *testing.T) {
	names := EnvNames()
	names[0] = "MUTATED"

	require.Equal(t, "CHARON_SECURITY_CROWDSEC_API_KEY", EnvNames()[0])
}

func TestFromEnvEachNameAlone(t *testing.T) {
	for _, name := range EnvNames() {
		t.Run(name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(name, "key-for-"+name)

			value, got := FromEnv()
			require.Equal(t, "key-for-"+name, value)
			require.Equal(t, name, got)
		})
	}
}

func TestFromEnvNoneSet(t *testing.T) {
	clearEnv(t)

	value, name := FromEnv()
	require.Empty(t, value)
	require.Empty(t, name)
}

// TestFromEnvEarlierNameWins sets every adjacent pair to different values and
// checks the earlier name in EnvNames() is returned.
func TestFromEnvEarlierNameWins(t *testing.T) {
	names := EnvNames()
	for i := 0; i+1 < len(names); i++ {
		earlier, later := names[i], names[i+1]
		t.Run(earlier+" over "+later, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(later, "later-key")
			t.Setenv(earlier, "earlier-key")

			value, name := FromEnv()
			require.Equal(t, "earlier-key", value)
			require.Equal(t, earlier, name)
		})
	}
}

// TestFromEnvSkipsEmptyValues checks that an empty earlier variable does not
// shadow a later one.
func TestFromEnvSkipsEmptyValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CROWDSEC_API_KEY", "fallback-key")

	value, name := FromEnv()
	require.Equal(t, "fallback-key", value)
	require.Equal(t, "CROWDSEC_API_KEY", name)
}

// TestFromEnvOrder pins the cross-package precedence: CHARON_SECURITY_CROWDSEC_API_KEY
// wins over CROWDSEC_API_KEY, which wins over CROWDSEC_BOUNCER_API_KEY.
func TestFromEnvOrder(t *testing.T) {
	clearEnv(t)
	t.Setenv("CROWDSEC_BOUNCER_API_KEY", "bouncer")
	t.Setenv("CROWDSEC_API_KEY", "primary")
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "charon")

	value, name := FromEnv()
	require.Equal(t, "charon", value)
	require.Equal(t, "CHARON_SECURITY_CROWDSEC_API_KEY", name)

	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "")
	value, name = FromEnv()
	require.Equal(t, "primary", value)
	require.Equal(t, "CROWDSEC_API_KEY", name)
}

func TestReadFileTrimming(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{name: "key with newline", content: "test-key-123\n", expected: "test-key-123"},
		{name: "key with extra whitespace", content: "  test-key-456  \n", expected: "test-key-456"},
		{name: "key without newline", content: "test-key-789", expected: "test-key-789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPath := filepath.Join(tmpDir, filepath.Base(tt.expected))
			require.NoError(t, os.WriteFile(keyPath, []byte(tt.content), 0o600))

			require.Equal(t, tt.expected, ReadFile(keyPath))
		})
	}
}

func TestReadFileMissingFile(t *testing.T) {
	t.Parallel()

	require.Empty(t, ReadFile(filepath.Join(t.TempDir(), "nonexistent")))
}

func TestReadFileDirectory(t *testing.T) {
	t.Parallel()

	require.Empty(t, ReadFile(t.TempDir()))
}

func TestReadFileWhitespaceOnly(t *testing.T) {
	t.Parallel()

	keyPath := filepath.Join(t.TempDir(), "bouncer_key")
	require.NoError(t, os.WriteFile(keyPath, []byte(" \n\t"), 0o600))

	require.Empty(t, ReadFile(keyPath))
}

// Keys used by the decision-matrix tests. The fake LAPI accepts only
// testSavedValid and testEnvValid; every other key is rejected.
const (
	testSavedValid   = "saved-valid-key-0001"
	testSavedInvalid = "saved-invalid-key-001"
	testEnvValid     = "env-valid-key-000001"
	testEnvInvalid   = "env-invalid-key-00001"
	testRegistered   = "registered-key-000001"
)

// fakeLAPI records every validate, register and save call Ensure makes.
type fakeLAPI struct {
	valid     map[string]bool
	validated []string
	registers int
	regErr    error
	saved     []string
	saveErr   error
}

func newFakeLAPI() *fakeLAPI {
	return &fakeLAPI{valid: map[string]bool{testSavedValid: true, testEnvValid: true}}
}

func (f *fakeLAPI) validate(_ context.Context, key string) bool {
	f.validated = append(f.validated, key)
	return f.valid[key]
}

func (f *fakeLAPI) register(_ context.Context) (string, error) {
	f.registers++
	if f.regErr != nil {
		return "", f.regErr
	}
	return testRegistered, nil
}

// save mirrors the real writer: it records the call and, unless a failure is
// injected, stores the key so later ReadFile calls see it.
func (f *fakeLAPI) save(path, key string) error {
	f.saved = append(f.saved, key)
	if f.saveErr != nil {
		return f.saveErr
	}
	return os.WriteFile(path, []byte(key+"\n"), 0o600)
}

// seedKeyFile writes saved into a fresh key file and returns its path.
// An empty saved value leaves the file absent.
func seedKeyFile(t *testing.T, saved string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bouncer_key")
	if saved != "" {
		require.NoError(t, os.WriteFile(path, []byte(saved+"\n"), 0o600))
	}
	return path
}

func setOnlyEnvKey(t *testing.T, env string) {
	t.Helper()
	clearEnv(t)
	if env != "" {
		t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", env)
	}
}

// TestEnsureDecisionMatrix covers the 11 reachable rows of the saved-key x
// env-key decision table: saved {none, valid, invalid} x env {none, same as
// saved, valid different, invalid different}.
func TestEnsureDecisionMatrix(t *testing.T) {
	rows := []struct {
		name          string
		saved         string
		env           string
		wantOutcome   Outcome
		wantValidated []string
		wantRegisters int
		wantSaved     []string
		wantKey       string
		wantIgnored   bool
		wantRejected  bool
		wantEnvKey    string
	}{
		{
			name: "saved none, env none", saved: "", env: "",
			wantOutcome: OutcomeRegistered, wantRegisters: 1,
			wantSaved: []string{testRegistered}, wantKey: testRegistered,
		},
		{
			name: "saved none, env valid", saved: "", env: testEnvValid,
			wantOutcome: OutcomeEnvKeySaved, wantValidated: []string{testEnvValid},
			wantSaved: []string{testEnvValid}, wantKey: testEnvValid, wantEnvKey: testEnvValid,
		},
		{
			name: "saved none, env invalid", saved: "", env: testEnvInvalid,
			wantOutcome: OutcomeRegistered, wantValidated: []string{testEnvInvalid},
			wantRegisters: 1, wantSaved: []string{testRegistered}, wantKey: testRegistered,
			wantRejected: true, wantEnvKey: testEnvInvalid,
		},
		{
			name: "saved valid, env none", saved: testSavedValid, env: "",
			wantOutcome: OutcomeSavedKeyValid, wantValidated: []string{testSavedValid},
			wantKey: testSavedValid,
		},
		{
			name: "saved valid, env same", saved: testSavedValid, env: testSavedValid,
			wantOutcome: OutcomeSavedKeyValid, wantValidated: []string{testSavedValid},
			wantKey: testSavedValid, wantEnvKey: testSavedValid,
		},
		{
			name: "saved valid, env valid different", saved: testSavedValid, env: testEnvValid,
			wantOutcome: OutcomeSavedKeyValid, wantValidated: []string{testSavedValid},
			wantKey: testSavedValid, wantIgnored: true, wantEnvKey: testEnvValid,
		},
		{
			name: "saved valid, env invalid different", saved: testSavedValid, env: testEnvInvalid,
			wantOutcome: OutcomeSavedKeyValid, wantValidated: []string{testSavedValid},
			wantKey: testSavedValid, wantIgnored: true, wantEnvKey: testEnvInvalid,
		},
		{
			name: "saved invalid, env none", saved: testSavedInvalid, env: "",
			wantOutcome: OutcomeRegistered, wantValidated: []string{testSavedInvalid},
			wantRegisters: 1, wantSaved: []string{testRegistered}, wantKey: testRegistered,
		},
		{
			name: "saved invalid, env same", saved: testSavedInvalid, env: testSavedInvalid,
			wantOutcome: OutcomeRegistered, wantValidated: []string{testSavedInvalid},
			wantRegisters: 1, wantSaved: []string{testRegistered}, wantKey: testRegistered,
			wantRejected: true, wantEnvKey: testSavedInvalid,
		},
		{
			name: "saved invalid, env valid different", saved: testSavedInvalid, env: testEnvValid,
			wantOutcome: OutcomeEnvKeySaved, wantValidated: []string{testSavedInvalid, testEnvValid},
			wantSaved: []string{testEnvValid}, wantKey: testEnvValid, wantEnvKey: testEnvValid,
		},
		{
			name: "saved invalid, env invalid different", saved: testSavedInvalid, env: testEnvInvalid,
			wantOutcome: OutcomeRegistered, wantValidated: []string{testSavedInvalid, testEnvInvalid},
			wantRegisters: 1, wantSaved: []string{testRegistered}, wantKey: testRegistered,
			wantRejected: true, wantEnvKey: testEnvInvalid,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			path := seedKeyFile(t, row.saved)
			setOnlyEnvKey(t, row.env)
			fake := newFakeLAPI()

			res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

			require.NoError(t, err)
			require.NoError(t, res.SaveErr)
			require.Equal(t, row.wantOutcome, res.Outcome, "outcome")
			require.Equal(t, row.wantKey, res.Key, "returned key")
			require.Equal(t, row.wantValidated, fake.validated, "validate calls")
			require.Equal(t, row.wantRegisters, fake.registers, "register calls")
			require.Equal(t, row.wantSaved, fake.saved, "save calls")
			require.Equal(t, row.wantIgnored, res.EnvKeyIgnored, "EnvKeyIgnored")
			require.Equal(t, row.wantRejected, res.EnvKeyRejected, "EnvKeyRejected")
			require.Equal(t, row.wantEnvKey, res.EnvKey, "EnvKey")
			require.Equal(t, row.wantKey, ReadFile(path), "key file after Ensure")
		})
	}
}

func TestEnsureSaveFailureAfterEnvValidation(t *testing.T) {
	path := seedKeyFile(t, "")
	setOnlyEnvKey(t, testEnvValid)
	fake := newFakeLAPI()
	fake.saveErr = errors.New("disk full")

	res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

	require.NoError(t, err)
	require.ErrorContains(t, res.SaveErr, "disk full")
	require.Equal(t, OutcomeEnvKeySaved, res.Outcome)
	require.Equal(t, testEnvValid, res.Key)
	require.Equal(t, 0, fake.registers)
}

func TestEnsureSaveFailureAfterRegistration(t *testing.T) {
	path := seedKeyFile(t, "")
	setOnlyEnvKey(t, testEnvInvalid)
	fake := newFakeLAPI()
	fake.saveErr = errors.New("read-only file system")

	res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

	require.NoError(t, err)
	require.ErrorContains(t, res.SaveErr, "read-only file system")
	require.Equal(t, OutcomeRegistered, res.Outcome)
	require.Equal(t, testRegistered, res.Key)
	require.True(t, res.EnvKeyRejected)
}

func TestEnsureRegisterFailureIsReturned(t *testing.T) {
	path := seedKeyFile(t, testSavedInvalid)
	setOnlyEnvKey(t, testEnvInvalid)
	fake := newFakeLAPI()
	registerErr := errors.New("cscli unavailable")
	fake.regErr = registerErr

	res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

	require.ErrorIs(t, err, registerErr)
	require.Empty(t, res.Key)
	require.Empty(t, fake.saved)
	// The rejection is still reported so the handler can show it.
	require.True(t, res.EnvKeyRejected)
	require.Equal(t, testEnvInvalid, res.EnvKey)
}

func TestResolvePrefersFileOverEnv(t *testing.T) {
	path := seedKeyFile(t, testSavedValid)
	setOnlyEnvKey(t, testEnvValid)

	key, src := Resolve(path)
	require.Equal(t, testSavedValid, key)
	require.Equal(t, SourceFile, src)
}

func TestResolveFallsBackToEnv(t *testing.T) {
	path := seedKeyFile(t, "")
	setOnlyEnvKey(t, testEnvValid)

	key, src := Resolve(path)
	require.Equal(t, testEnvValid, key)
	require.Equal(t, SourceEnv, src)
}

func TestResolveWhitespaceFileFallsBackToEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bouncer_key")
	require.NoError(t, os.WriteFile(path, []byte(" \n"), 0o600))
	setOnlyEnvKey(t, testEnvValid)

	key, src := Resolve(path)
	require.Equal(t, testEnvValid, key)
	require.Equal(t, SourceEnv, src)
}

func TestResolveNone(t *testing.T) {
	setOnlyEnvKey(t, "")

	key, src := Resolve(seedKeyFile(t, ""))
	require.Empty(t, key)
	require.Equal(t, SourceNone, src)
}
