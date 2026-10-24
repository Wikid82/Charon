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
	"strings"
)

// DefaultFile is the saved bouncer key location inside the container.
const DefaultFile = "/app/data/crowdsec/bouncer_key"

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
)

// Resolve returns the key the proxy uses: the saved key file first, then the
// first non-empty key environment variable.
func Resolve(path string) (key string, src Source) {
	if key := ReadFile(path); key != "" {
		return key, SourceFile
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
	// SaveErr is a failed save of Key. The key is still valid for the session.
	SaveErr error
}

// Ensure decides which bouncer key to use and makes the key file match it.
//
// Rules, in order:
//  1. A saved key that validates is kept. The env value is not tested.
//  2. Otherwise an env value that differs from the saved key and validates is saved.
//  3. Otherwise a new bouncer is registered and its key saved. An env value
//     equal to the saved key is not validated again; it counts as rejected.
//
// A save failure after rule 2 or 3 is reported in EnsureResult.SaveErr, not as
// the error. A register failure is returned as the error, with the rejection
// facts still filled in. Ensure does not log; callers do.
func Ensure(
	ctx context.Context,
	path string,
	validate func(context.Context, string) bool,
	register func(context.Context) (string, error),
	save func(path, key string) error,
) (EnsureResult, error) {
	env, _ := FromEnv()
	saved := ReadFile(path)
	res := EnsureResult{EnvKey: env}

	if saved != "" && validate(ctx, saved) {
		res.Key = saved
		res.Outcome = OutcomeSavedKeyValid
		res.EnvKeyIgnored = env != "" && env != saved
		return res, nil
	}

	if env != "" && env != saved && validate(ctx, env) {
		res.Key = env
		res.Outcome = OutcomeEnvKeySaved
		res.SaveErr = save(path, env)
		return res, nil
	}

	res.EnvKeyRejected = env != ""
	newKey, err := register(ctx)
	if err != nil {
		return res, fmt.Errorf("register bouncer: %w", err)
	}
	res.Key = newKey
	res.Outcome = OutcomeRegistered
	res.SaveErr = save(path, newKey)
	return res, nil
}

// SaveFailedNotice says the new key could not be written to path. The new key is held nowhere else, so
// CrowdSec blocking depends on the key file becoming writable.
func SaveFailedNotice(path string) string {
	return "Charon could not save the new key to " + path +
		"; CrowdSec blocking may not work until the key file can be written. " +
		"Fix write access to the data directory and restart CrowdSec."
}

// RegisterFailedNotice says no replacement key could be registered, so no new key was saved to path.
func RegisterFailedNotice(path string) string {
	return "Charon could not register a new key with CrowdSec, so no new key was saved to " + path +
		". CrowdSec blocking may not work until a key can be registered."
}
