package bouncerkey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Keys for the rows where LAPI could not give a verdict. The fake answers Unknown for them.
const (
	testSavedUnknown = "saved-unknown-key-0001"
	testEnvUnknown   = "env-unknown-key-00001"
)

// TestEnsureUnknownNeverRegistersOrSaves covers every reachable row in which a present
// candidate got VerdictUnknown. Such a row must keep the saved key (or the env key when
// no file exists), must not register, delete or save, and must not test candidates
// after the unknown one.
func TestEnsureUnknownNeverRegistersOrSaves(t *testing.T) {
	rows := []struct {
		name          string
		saved         string
		env           string
		wantKey       string
		wantValidated []string
	}{
		{
			name: "saved unknown, env none", saved: testSavedUnknown, env: "",
			wantKey: testSavedUnknown, wantValidated: []string{testSavedUnknown},
		},
		{
			name: "saved unknown, env valid different", saved: testSavedUnknown, env: testEnvValid,
			wantKey: testSavedUnknown, wantValidated: []string{testSavedUnknown},
		},
		{
			name: "saved unknown, env invalid different", saved: testSavedUnknown, env: testEnvInvalid,
			wantKey: testSavedUnknown, wantValidated: []string{testSavedUnknown},
		},
		{
			name: "saved unknown, env unknown different", saved: testSavedUnknown, env: testEnvUnknown,
			wantKey: testSavedUnknown, wantValidated: []string{testSavedUnknown},
		},
		{
			name: "saved unknown, env same", saved: testSavedUnknown, env: testSavedUnknown,
			wantKey: testSavedUnknown, wantValidated: []string{testSavedUnknown},
		},
		{
			name: "saved invalid, env unknown", saved: testSavedInvalid, env: testEnvUnknown,
			wantKey: testSavedInvalid, wantValidated: []string{testSavedInvalid, testEnvUnknown},
		},
		{
			name: "saved none, env unknown", saved: "", env: testEnvUnknown,
			wantKey: testEnvUnknown, wantValidated: []string{testEnvUnknown},
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			path := seedKeyFile(t, row.saved)
			setOnlyEnvKey(t, row.env)
			fake := newFakeLAPI()
			fake.unknown = map[string]bool{testSavedUnknown: true, testEnvUnknown: true}

			res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

			require.NoError(t, err)
			require.NoError(t, res.SaveErr)
			require.Equal(t, OutcomeUnverified, res.Outcome, "outcome")
			require.Equal(t, row.wantKey, res.Key, "returned key")
			require.Equal(t, row.wantValidated, fake.validated, "validate calls")
			require.Zero(t, fake.registers, "an unknown verdict must never register")
			require.Empty(t, fake.saved, "an unknown verdict must never save")
			require.False(t, res.EnvKeyRejected, "an unknown verdict is not a rejection")
			require.Equal(t, row.env, res.EnvKey, "EnvKey")
			require.Equal(t, row.saved, ReadFile(path), "key file is unchanged")
		})
	}
}

// TestEnsureDefinitelyRejectedCandidatesStillRegister pins the destructive rule from the
// other side: every present candidate rejected with a definite answer still registers.
func TestEnsureDefinitelyRejectedCandidatesStillRegister(t *testing.T) {
	path := seedKeyFile(t, testSavedInvalid)
	setOnlyEnvKey(t, testEnvInvalid)
	fake := newFakeLAPI()

	res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

	require.NoError(t, err)
	require.Equal(t, OutcomeRegistered, res.Outcome)
	require.Equal(t, 1, fake.registers)
	require.Equal(t, []string{testRegistered}, fake.saved)
}

// TestEnsureCancelledContextNeverRegisters covers a shutdown during the decision. The
// validators here answer Rejected, so only the context guard can keep the destructive
// step from running.
func TestEnsureCancelledContextNeverRegisters(t *testing.T) {
	rows := []struct {
		name    string
		saved   string
		env     string
		wantKey string
	}{
		{name: "saved rejected, env rejected", saved: testSavedInvalid, env: testEnvInvalid, wantKey: testSavedInvalid},
		{name: "saved none, env rejected", saved: "", env: testEnvInvalid, wantKey: ""},
		{name: "nothing present", saved: "", env: "", wantKey: ""},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			path := seedKeyFile(t, row.saved)
			setOnlyEnvKey(t, row.env)
			fake := newFakeLAPI()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			res, err := Ensure(ctx, path, fake.validate, fake.register, fake.save)

			require.NoError(t, err)
			require.Equal(t, OutcomeUnverified, res.Outcome)
			require.Equal(t, row.wantKey, res.Key)
			require.Zero(t, fake.registers, "a cancelled context must never register")
			require.Empty(t, fake.saved, "a cancelled context must never save")
			require.False(t, res.EnvKeyRejected)
		})
	}
}
