// Package bouncerkey holds the lookup rules for the CrowdSec bouncer API key
// shared by the proxy config, the HTTP handlers and the startup reconciliation.
//
// It is a leaf package (standard library only) so that every caller can import
// it without creating an import cycle.
package bouncerkey

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultFile is the saved bouncer key location inside the container.
const DefaultFile = "/app/data/crowdsec/bouncer_key"

// PathEnvName is the environment variable that overrides the key file location.
const PathEnvName = "CHARON_CROWDSEC_BOUNCER_KEY_PATH"

// keyFileName is the key file name inside a data directory.
const keyFileName = "bouncer_key"

// PathFor returns the key file location: the PathEnvName override when set, else
// <dataDir>/bouncer_key when dataDir is set, else DefaultFile.
func PathFor(dataDir string) string {
	if override := strings.TrimSpace(os.Getenv(PathEnvName)); override != "" {
		return filepath.Clean(override)
	}
	if dataDir != "" {
		return filepath.Join(dataDir, keyFileName)
	}
	return DefaultFile
}

// heldKeys maps a key file path to a key that could not be saved to that path.
// It lives for the process; see Hold.
var (
	heldMu   sync.RWMutex
	heldKeys = map[string]string{}
)

// Hold remembers key for path after a save to path failed, so readers in this
// process keep using the key that CrowdSec accepted.
//
// The hold is in memory only. A restart drops it: the next boot checks the key file
// again and registers a new bouncer when the file key is rejected, and the Error log
// and the status notice say that the key is not saved.
func Hold(path, key string) {
	heldMu.Lock()
	defer heldMu.Unlock()
	heldKeys[path] = key
}

// Held returns the key held for path, or "" when none is held.
func Held(path string) string {
	heldMu.RLock()
	defer heldMu.RUnlock()
	return heldKeys[path]
}

// Release drops any key held for path.
func Release(path string) {
	heldMu.Lock()
	defer heldMu.Unlock()
	delete(heldKeys, path)
}

// releaseIf drops the hold for path only while it still equals key. Callers read the held key, check it
// against the key file or a validator, and then release it; a concurrent Hold of a different key in that
// window must survive, so an unconditional Release is not safe here.
func releaseIf(path, key string) {
	heldMu.Lock()
	defer heldMu.Unlock()
	if heldKeys[path] == key {
		delete(heldKeys, path)
	}
}

// envNames lists the environment variables that may carry the bouncer key,
// in lookup order (the first non-empty value wins).
var envNames = [...]string{
	"CHARON_SECURITY_CROWDSEC_API_KEY",
	"CROWDSEC_API_KEY",
	"CROWDSEC_BOUNCER_API_KEY",
	"CERBERUS_SECURITY_CROWDSEC_API_KEY",
	"CPM_SECURITY_CROWDSEC_API_KEY",
}

// EnvNames returns the key environment variable names in lookup order.
// The returned slice is a copy and may be modified by the caller.
func EnvNames() []string {
	return append([]string(nil), envNames[:]...)
}

// FromEnv returns the first non-empty key environment variable and its name.
// It returns empty strings when none is set.
func FromEnv() (value, name string) {
	for _, n := range envNames {
		if v := os.Getenv(n); v != "" {
			return v, n
		}
	}
	return "", ""
}

// ReadFile returns the trimmed content of the key file at path. Any read error
// (missing file, directory, permission) yields an empty string.
func ReadFile(path string) string {
	// #nosec G304 -- path is DefaultFile or a caller-supplied location chosen by the server, never user input
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Source names where Resolve found the key.
type Source string

const (
	// SourceFile means the key came from the saved key file.
	SourceFile Source = "file"
	// SourceEnv means the key came from a key environment variable.
	SourceEnv Source = "env"
	// SourceNone means no key is available.
	SourceNone Source = "none"
	// SourceMemory means the key is held in memory because it could not be saved.
	SourceMemory Source = "memory"
)

// Resolve returns the key the proxy uses: a key held in memory, then the saved key
// file, then the first non-empty key environment variable.
//
// A held key wins while the file does not hold the same key, including when the file is
// empty. Once the file holds it, the hold is dropped and the file is the source. Between
// Ensure runs this is the only check, since only Ensure can validate a key.
func Resolve(path string) (key string, src Source) {
	file := ReadFile(path)
	if held := Held(path); held != "" {
		if held != file {
			return held, SourceMemory
		}
		releaseIf(path, held)
	}
	if file != "" {
		return file, SourceFile
	}
	if val, _ := FromEnv(); val != "" {
		return val, SourceEnv
	}
	return "", SourceNone
}

// Outcome describes which branch of Ensure produced the key.
type Outcome int

const (
	// OutcomeUnset is the zero value; Ensure never returns it on success.
	OutcomeUnset Outcome = iota
	// OutcomeSavedKeyValid means the saved key passed validation and is kept.
	OutcomeSavedKeyValid
	// OutcomeEnvKeySaved means a valid env key replaced a missing or invalid saved key.
	OutcomeEnvKeySaved
	// OutcomeRegistered means a new bouncer was registered with LAPI.
	OutcomeRegistered
	// OutcomeUnverified means LAPI could not give a verdict on a present key. The key is kept as it
	// was, nothing is registered, deleted or saved, and the caller reports it as unverified.
	OutcomeUnverified
	// OutcomeHeldKeyValid means the key held in memory after a failed save validated. Nothing is
	// registered, deleted or saved except one best-effort save of the held key.
	OutcomeHeldKeyValid
)

// EnsureResult reports the key chosen by Ensure and the facts callers need to
// log and to update their state.
type EnsureResult struct {
	// Key is the key the proxy will use after Ensure returns.
	Key string
	// Outcome is the branch that produced Key.
	Outcome Outcome
	// EnvKey is the first non-empty key environment value seen, if any.
	EnvKey string
	// EnvKeyIgnored is true when an env value differs from the saved key and the saved key was kept.
	EnvKeyIgnored bool
	// EnvKeyRejected is true when an env value was present but LAPI did not accept it.
	EnvKeyRejected bool
	// SaveErr is a failed save of Key. Nothing else holds the key, so readers of the key file do not see it.
	SaveErr error
	// HeldSaveErr is the failed best-effort save of a held key that validated (OutcomeHeldKeyValid). It is
	// reported at Info, not Error, because the key is already in use from memory.
	HeldSaveErr error
}

// Ensure decides which bouncer key to use and makes the key file match it.
//
// validate returns a three-valued Verdict. Only a definite VerdictRejected may lead to a
// new bouncer; VerdictUnknown (LAPI unreachable, slow or erroring, or the context ended)
// is never evidence against a key.
//
// Rules, in order:
//  0. A key held in memory after a failed save is tested first, unless the file already
//     holds it (the hold is then dropped). Accepted: OutcomeHeldKeyValid, with one best-effort
//     save that releases the hold on success and reports a failure in HeldSaveErr only.
//     Unknown: OutcomeUnverified with the held key. Rejected: the hold is dropped and rules
//     1-3 run as if nothing were held.
//  1. A saved key that validates is kept. The env value is not tested.
//     An unknown saved key stops the decision as OutcomeUnverified.
//  2. Otherwise an env value that differs from the saved key and validates is saved.
//     An unknown env value stops the decision as OutcomeUnverified.
//  3. Otherwise a new bouncer is registered and its key saved. This runs only when every
//     present candidate was definitively rejected. A cancelled ctx also stops here as
//     OutcomeUnverified, so a shutdown never rotates a key. An env value equal to the saved
//     key is not validated again; it counts as rejected.
//
// OutcomeUnverified changes nothing: no register, delete or save, and no rejection facts.
// Its Key is the saved key, or the unknown env value when no saved key exists.
//
// A save failure after rule 2 or 3 is reported in EnsureResult.SaveErr, not as
// the error, and the new key is held in memory so the proxy keeps using it. A
// register failure is returned as the error, with the rejection facts still
// filled in. Ensure does not log; callers do.
func Ensure(
	ctx context.Context,
	path string,
	validate func(context.Context, string) Verdict,
	register func(context.Context) (string, error),
	save func(path, key string) error,
) (EnsureResult, error) {
	env, _ := FromEnv()
	saved := ReadFile(path)
	res := EnsureResult{EnvKey: env}

	if held := Held(path); held != "" {
		if held == saved {
			releaseIf(path, held)
		} else {
			switch validate(ctx, held) {
			case VerdictAccepted:
				return heldKeyValid(res, path, held, save), nil
			case VerdictUnknown:
				res.Key = held
				res.Outcome = OutcomeUnverified
				return res, nil
			default:
				releaseIf(path, held)
			}
		}
	}

	if saved != "" {
		switch validate(ctx, saved) {
		case VerdictAccepted:
			res.Key = saved
			res.Outcome = OutcomeSavedKeyValid
			res.EnvKeyIgnored = env != "" && env != saved
			return res, nil
		case VerdictUnknown:
			return unverified(res, saved, ""), nil
		}
	}

	if env != "" && env != saved {
		switch validate(ctx, env) {
		case VerdictAccepted:
			res.Key = env
			res.Outcome = OutcomeEnvKeySaved
			res.SaveErr = save(path, env)
			settleNewKey(path, &res)
			return res, nil
		case VerdictUnknown:
			return unverified(res, saved, env), nil
		}
	}

	// Every present candidate was definitively rejected here. A cancelled context is not
	// evidence about the key, so it must not reach the destructive register step.
	if ctx.Err() != nil {
		return unverified(res, saved, ""), nil
	}

	res.EnvKeyRejected = env != ""
	newKey, err := register(ctx)
	if err != nil {
		return res, fmt.Errorf("register bouncer: %w", err)
	}
	res.Key = newKey
	res.Outcome = OutcomeRegistered
	res.SaveErr = save(path, newKey)
	settleNewKey(path, &res)
	return res, nil
}

// settleNewKey records a newly chosen key: a failed save holds it in memory, a successful save releases a
// hold on that same key.
func settleNewKey(path string, res *EnsureResult) {
	if res.SaveErr != nil {
		Hold(path, res.Key)
		return
	}
	releaseIf(path, res.Key)
}

// heldKeyValid finishes an Ensure result whose held key validated. The key is in use from memory either way.
// One best-effort save writes it to the key file; a success releases the hold. Nothing is registered or
// deleted, and a failed save is reported in HeldSaveErr only.
func heldKeyValid(res EnsureResult, path, held string, save func(path, key string) error) EnsureResult {
	res.Key = held
	res.Outcome = OutcomeHeldKeyValid
	res.HeldSaveErr = save(path, held)
	if res.HeldSaveErr == nil {
		releaseIf(path, held)
	}
	return res
}

// unverified finishes an Ensure result whose key could not be checked. The saved key stays in
// use; with no saved key, the unknown env value is the key the proxy reads.
func unverified(res EnsureResult, saved, unknownEnv string) EnsureResult {
	res.Outcome = OutcomeUnverified
	res.Key = saved
	if res.Key == "" {
		res.Key = unknownEnv
	}
	return res
}

// SaveFailedNotice says the new key could not be written to path. It is held in memory, so it is used until
// the process restarts; after a restart the key file must be writable again.
func SaveFailedNotice(path string) string {
	return "Charon could not save the new key to " + path +
		". It uses the new key until it restarts; after a restart CrowdSec blocking may not work until the key file can be written. " +
		"Fix write access to the data directory and restart CrowdSec."
}

// HeldKeyInUseMessage says a key held in memory validated and is in use. saved reports whether this check
// wrote it to the key file. Callers log it at Info; it names no key material.
func HeldKeyInUseMessage(saved bool) string {
	if saved {
		return "CrowdSec bouncer: the key held in memory was saved to the key file and is in use"
	}
	return "CrowdSec bouncer: the key held in memory is still in use; the key file could not be written"
}

// RegisterFailedNotice says no replacement key could be registered, so no new key was saved to path.
func RegisterFailedNotice(path string) string {
	return "Charon could not register a new key with CrowdSec, so no new key was saved to " + path +
		". CrowdSec blocking may not work until a key can be registered."
}

// UnverifiedMessage says a saved key could not be checked, so it was kept unchanged. It is the one
// wording for every caller (Start warning, handler and startup logs); it names no key material.
const UnverifiedMessage = "CrowdSec could not be reached to check the key, so Charon keeps the current key " +
	"and did not register a new one. It checks again at the next start."
