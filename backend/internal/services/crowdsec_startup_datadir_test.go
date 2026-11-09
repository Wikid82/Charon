package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/models"
)

// dirRecordingExecutor is a CrowdsecProcessManager that records the data directory each call receives.
type dirRecordingExecutor struct {
	startPid  int
	startDir  string
	statusDir string
	started   bool
}

func (e *dirRecordingExecutor) Start(_ context.Context, _, configDir string) (int, error) {
	e.started = true
	e.startDir = configDir
	return e.startPid, nil
}

func (e *dirRecordingExecutor) Stop(_ context.Context, _ string) error { return nil }

func (e *dirRecordingExecutor) Status(_ context.Context, configDir string) (running bool, pid int, err error) {
	e.statusDir = configDir
	if e.started {
		return true, e.startPid, nil
	}
	return false, 0, nil
}

// TestEnsureBouncerRegistrationOnStartupUsesDataDirKeyPath pins the boot key path (#1719): the key file is
// <dataDir>/bouncer_key, the same file the handlers and the proxy use, not the image default.
func TestEnsureBouncerRegistrationOnStartupUsesDataDirKeyPath(t *testing.T) {
	h := newBouncerHarness(t)
	t.Setenv(bouncerkey.PathEnvName, "")
	dataDir := filepath.Join(h.dataDir, "crowdsec")

	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey)}
	require.NoError(t, ensureBouncerRegistrationOnStartup(dataDir, exec))

	require.Equal(t, testGeneratedKey, bouncerkey.ReadFile(filepath.Join(dataDir, "bouncer_key")))
}

// TestEnsureBouncerRegistrationOnStartupHonorsKeyPathOverride checks the override wins over the data directory at boot.
func TestEnsureBouncerRegistrationOnStartupHonorsKeyPathOverride(t *testing.T) {
	h := newBouncerHarness(t)
	override := filepath.Join(h.dataDir, "override", "alt_bouncer_key")
	t.Setenv(bouncerkey.PathEnvName, override)
	dataDir := filepath.Join(h.dataDir, "crowdsec")

	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey)}
	require.NoError(t, ensureBouncerRegistrationOnStartup(dataDir, exec))

	require.Equal(t, testGeneratedKey, bouncerkey.ReadFile(override))
	_, statErr := os.Stat(filepath.Join(dataDir, "bouncer_key"))
	require.True(t, os.IsNotExist(statErr), "the data directory key file must not be written when the override is set")
}

// TestReconcileCrowdSecOnStartupRelativeDataDirEndToEnd runs boot reconciliation with a relative data directory
// (the non-container default): the engine receives the same relative directory, the whitelist and the key file
// live under it, and nothing is written under the absolute image path.
func TestReconcileCrowdSecOnStartupRelativeDataDirEndToEnd(t *testing.T) {
	h := newBouncerHarness(t)
	t.Setenv(bouncerkey.PathEnvName, "")
	shortenCrowdSecVerifyWait(t)
	t.Chdir(h.dataDir)

	relDataDir := filepath.Join("data", "crowdsec")
	require.NoError(t, os.MkdirAll(filepath.Join(relDataDir, "config"), 0o750))
	binPath := filepath.Join(h.dataDir, "crowdsec-bin")
	require.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o750)) // #nosec G306 -- executable test script

	db := setupCrowdsecTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, CrowdSecMode: "local"}).Error)

	engine := &dirRecordingExecutor{startPid: 4242}
	cmdExec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey)}

	ReconcileCrowdSecOnStartup(db, engine, binPath, relDataDir, cmdExec)

	require.True(t, engine.started, "reconciliation should start the engine")
	require.Equal(t, relDataDir, engine.startDir, "the engine must receive the configured data directory")
	require.Equal(t, relDataDir, engine.statusDir)
	require.Equal(t, testGeneratedKey, bouncerkey.ReadFile(filepath.Join(relDataDir, "bouncer_key")))
	require.Equal(t, testGeneratedKey, bouncerkey.ReadFile(filepath.Join(h.dataDir, relDataDir, "bouncer_key")))
}
