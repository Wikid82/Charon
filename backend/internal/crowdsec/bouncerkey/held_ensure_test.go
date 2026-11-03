package bouncerkey

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// Keys for the held-key rows. The fake accepts testHeldValid, answers Unknown for testHeldUnknown and
// rejects every other key.
const (
	testHeldValid   = "held-valid-key-0000001"
	testHeldUnknown = "held-unknown-key-000001"
	testHeldInvalid = "held-invalid-key-000001"
)

// newHeldFake returns the shared fake with the held keys added to its verdict tables.
func newHeldFake() *fakeLAPI {
	fake := newFakeLAPI()
	fake.valid[testHeldValid] = true
	fake.unknown = map[string]bool{testHeldUnknown: true}
	return fake
}

// TestEnsureHeldKeyMatrix covers a held key (candidate 0) against the saved and env keys. A held key that
// validates is kept and nothing is registered or deleted; one that is Unknown stops as OutcomeUnverified; one
// that is rejected is released and the saved and env candidates decide as they do without a hold.
func TestEnsureHeldKeyMatrix(t *testing.T) {
	rows := []struct {
		name          string
		held          string
		saved         string
		env           string
		wantOutcome   Outcome
		wantValidated []string
		wantRegisters int
		wantSaved     []string
		wantKey       string
		wantFile      string
		wantHeldAfter string
	}{
		{
			name: "held accepted, saved none, env none", held: testHeldValid,
			wantOutcome: OutcomeHeldKeyValid, wantValidated: []string{testHeldValid},
			wantSaved: []string{testHeldValid}, wantKey: testHeldValid, wantFile: testHeldValid,
		},
		{
			name: "held accepted, saved invalid, env valid different", held: testHeldValid,
			saved: testSavedInvalid, env: testEnvValid,
			wantOutcome: OutcomeHeldKeyValid, wantValidated: []string{testHeldValid},
			wantSaved: []string{testHeldValid}, wantKey: testHeldValid, wantFile: testHeldValid,
		},
		{
			name: "held accepted, saved valid (restored backup), env invalid", held: testHeldValid,
			saved: testSavedValid, env: testEnvInvalid,
			wantOutcome: OutcomeHeldKeyValid, wantValidated: []string{testHeldValid},
			wantSaved: []string{testHeldValid}, wantKey: testHeldValid, wantFile: testHeldValid,
		},
		{
			name: "held equals saved, accepted", held: testHeldValid, saved: testHeldValid,
			wantOutcome: OutcomeSavedKeyValid, wantValidated: []string{testHeldValid},
			wantKey: testHeldValid, wantFile: testHeldValid,
		},
		{
			name: "held unknown, saved valid, env valid", held: testHeldUnknown,
			saved: testSavedValid, env: testEnvValid,
			wantOutcome: OutcomeUnverified, wantValidated: []string{testHeldUnknown},
			wantKey: testHeldUnknown, wantFile: testSavedValid, wantHeldAfter: testHeldUnknown,
		},
		{
			name: "held unknown, saved none, env none", held: testHeldUnknown,
			wantOutcome: OutcomeUnverified, wantValidated: []string{testHeldUnknown},
			wantKey: testHeldUnknown, wantFile: "", wantHeldAfter: testHeldUnknown,
		},
		{
			name: "held rejected, saved valid, env none", held: testHeldInvalid, saved: testSavedValid,
			wantOutcome: OutcomeSavedKeyValid, wantValidated: []string{testHeldInvalid, testSavedValid},
			wantKey: testSavedValid, wantFile: testSavedValid,
		},
		{
			name: "held rejected, saved none, env valid", held: testHeldInvalid, env: testEnvValid,
			wantOutcome: OutcomeEnvKeySaved, wantValidated: []string{testHeldInvalid, testEnvValid},
			wantSaved: []string{testEnvValid}, wantKey: testEnvValid, wantFile: testEnvValid,
		},
		{
			name: "held rejected, saved none, env none", held: testHeldInvalid,
			wantOutcome: OutcomeRegistered, wantValidated: []string{testHeldInvalid},
			wantRegisters: 1, wantSaved: []string{testRegistered}, wantKey: testRegistered, wantFile: testRegistered,
		},
		{
			name: "held rejected, saved invalid, env invalid", held: testHeldInvalid,
			saved: testSavedInvalid, env: testEnvInvalid,
			wantOutcome: OutcomeRegistered, wantValidated: []string{testHeldInvalid, testSavedInvalid, testEnvInvalid},
			wantRegisters: 1, wantSaved: []string{testRegistered}, wantKey: testRegistered, wantFile: testRegistered,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			path := seedKeyFile(t, row.saved)
			setOnlyEnvKey(t, row.env)
			Hold(path, row.held)
			t.Cleanup(func() { Release(path) })
			fake := newHeldFake()

			res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

			require.NoError(t, err)
			require.NoError(t, res.SaveErr, "the save of a held key is reported as HeldSaveErr, not SaveErr")
			require.NoError(t, res.HeldSaveErr)
			require.Equal(t, row.wantOutcome, res.Outcome, "outcome")
			require.Equal(t, row.wantKey, res.Key, "returned key")
			require.Equal(t, row.wantValidated, fake.validated, "validate calls")
			require.Equal(t, row.wantRegisters, fake.registers, "register calls")
			require.Equal(t, row.wantSaved, fake.saved, "save calls")
			require.Equal(t, row.wantFile, ReadFile(path), "key file after Ensure")
			require.Equal(t, row.wantHeldAfter, Held(path), "held key after Ensure")
		})
	}
}

// TestEnsureHeldAcceptedSaveFailureKeepsHold: a held key that validates but cannot be written again is still
// used, nothing is registered or deleted, the failure is reported only as HeldSaveErr, and the hold stays.
func TestEnsureHeldAcceptedSaveFailureKeepsHold(t *testing.T) {
	path := seedKeyFile(t, testSavedInvalid)
	setOnlyEnvKey(t, "")
	Hold(path, testHeldValid)
	t.Cleanup(func() { Release(path) })
	fake := newHeldFake()
	fake.saveErr = errors.New("read-only file system")

	res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

	require.NoError(t, err)
	require.Equal(t, OutcomeHeldKeyValid, res.Outcome)
	require.Equal(t, testHeldValid, res.Key)
	require.ErrorContains(t, res.HeldSaveErr, "read-only file system")
	require.NoError(t, res.SaveErr)
	require.Zero(t, fake.registers)
	require.Equal(t, testHeldValid, Held(path), "a failed retry keeps the key in memory")
	require.Equal(t, testSavedInvalid, ReadFile(path), "the key file is unchanged")
}

// TestEnsureSaveFailureHoldsNewKey: a new key that cannot be saved (env replacement or registration) is held
// in memory, so the proxy keeps using it. A successful save leaves nothing held.
func TestEnsureSaveFailureHoldsNewKey(t *testing.T) {
	rows := []struct {
		name     string
		saved    string
		env      string
		wantHeld string
		wantOut  Outcome
	}{
		{name: "env replacement", saved: "", env: testEnvValid, wantHeld: testEnvValid, wantOut: OutcomeEnvKeySaved},
		{name: "registration", saved: testSavedInvalid, env: "", wantHeld: testRegistered, wantOut: OutcomeRegistered},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			path := seedKeyFile(t, row.saved)
			setOnlyEnvKey(t, row.env)
			t.Cleanup(func() { Release(path) })
			fake := newHeldFake()
			fake.saveErr = errors.New("read-only file system")

			res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

			require.NoError(t, err)
			require.Equal(t, row.wantOut, res.Outcome)
			require.ErrorContains(t, res.SaveErr, "read-only file system")
			require.Equal(t, row.wantHeld, Held(path))
			require.Equal(t, row.wantHeld, res.Key)
		})
	}
}

// TestEnsureSuccessfulSaveLeavesNothingHeld: a new key that is saved releases any hold for the path.
func TestEnsureSuccessfulSaveLeavesNothingHeld(t *testing.T) {
	path := seedKeyFile(t, "")
	setOnlyEnvKey(t, testEnvValid)
	t.Cleanup(func() { Release(path) })
	fake := newHeldFake()

	res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

	require.NoError(t, err)
	require.NoError(t, res.SaveErr)
	require.Equal(t, OutcomeEnvKeySaved, res.Outcome)
	require.Empty(t, Held(path))
}

// TestEnsureHeldUnknownDoesNotTestEnv: an Unknown held key stays held, and no env candidate is tested after it.
func TestEnsureHeldUnknownDoesNotTestEnv(t *testing.T) {
	path := seedKeyFile(t, "")
	setOnlyEnvKey(t, testEnvValid)
	Hold(path, testHeldUnknown)
	t.Cleanup(func() { Release(path) })
	fake := newHeldFake()

	res, err := Ensure(context.Background(), path, fake.validate, fake.register, fake.save)

	require.NoError(t, err)
	require.Equal(t, OutcomeUnverified, res.Outcome)
	require.Equal(t, []string{testHeldUnknown}, fake.validated)
	require.Equal(t, testHeldUnknown, Held(path))
}

// TestResolveHeldWinsOverFileAndEnv: while the file does not hold the same key, Resolve returns the held key
// from memory, ahead of both the file and the env value.
func TestResolveHeldWinsOverFileAndEnv(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{name: "file has another valid key", file: testSavedValid},
		{name: "file is empty", file: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := seedKeyFile(t, tc.file)
			setOnlyEnvKey(t, testEnvValid)
			Hold(path, testHeldValid)
			t.Cleanup(func() { Release(path) })

			key, src := Resolve(path)

			require.Equal(t, testHeldValid, key)
			require.Equal(t, SourceMemory, src)
			require.Equal(t, testHeldValid, Held(path), "a different file does not drop the hold")
		})
	}
}

// TestResolveReleasesHeldOnceFileMatches: once the key file holds the held key, the file is the source and the
// in-memory entry is dropped.
func TestResolveReleasesHeldOnceFileMatches(t *testing.T) {
	path := seedKeyFile(t, testHeldValid)
	setOnlyEnvKey(t, testEnvValid)
	Hold(path, testHeldValid)

	key, src := Resolve(path)

	require.Equal(t, testHeldValid, key)
	require.Equal(t, SourceFile, src)
	require.Empty(t, Held(path))
}

// TestSaveFailedNoticeWording pins the reworded notice shared by the handler, the startup log and the banner.
func TestSaveFailedNoticeWording(t *testing.T) {
	const path = "/app/data/crowdsec/bouncer_key"

	require.Equal(t,
		"Charon could not save the new key to "+path+
			". It uses the new key until it restarts; after a restart CrowdSec blocking may not work until the key file can be written. "+
			"Fix write access to the data directory and restart CrowdSec.",
		SaveFailedNotice(path))
}

// TestReleaseRacesHoldAndResolve runs Hold, Resolve and Release from many goroutines on one path while the
// file is rewritten. It must pass under -race, and every result must be a key some worker held, the file key,
// or nothing.
func TestReleaseRacesHoldAndResolve(t *testing.T) {
	t.Parallel()

	const workers = 12
	const rounds = 150
	path := seedKeyFile(t, "")
	const fileKey = "race-file-key-0000000000000001"

	var wg sync.WaitGroup
	for w := range workers {
		heldKey := fmt.Sprintf("race-held-key-%02d-0000000000000000", w)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				Hold(path, heldKey)
				key, src := Resolve(path)
				switch src {
				case SourceMemory:
					if !strings.HasPrefix(key, "race-held-key-") {
						t.Errorf("memory source returned %q", key)
					}
				case SourceFile:
					if key != fileKey {
						t.Errorf("file source returned %q", key)
					}
				}
				Release(path)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range rounds {
			_ = os.WriteFile(path, []byte(fileKey+"\n"), 0o600)
			_ = os.Remove(path)
		}
	}()
	wg.Wait()

	Release(path)
	require.Empty(t, Held(path))
}
