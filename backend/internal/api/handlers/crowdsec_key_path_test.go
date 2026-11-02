package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

// TestBouncerKeyPathOverrideWinsOverDataDir pins the precedence of the key file path (#1719): the
// CHARON_CROWDSEC_BOUNCER_KEY_PATH override wins even when a data directory is set, because boot and the
// proxy resolve the same override.
func TestBouncerKeyPathOverrideWinsOverDataDir(t *testing.T) {
	override := t.TempDir() + "/override_key"
	t.Setenv(bouncerkey.PathEnvName, override)

	h := &CrowdsecHandler{DataDir: t.TempDir()}
	require.Equal(t, override, h.bouncerKeyPath())
}

// TestBouncerKeyPathUsesDataDirWithoutOverride checks the data directory is used when no override is set.
func TestBouncerKeyPathUsesDataDirWithoutOverride(t *testing.T) {
	t.Setenv(bouncerkey.PathEnvName, "")
	dataDir := t.TempDir()

	h := &CrowdsecHandler{DataDir: dataDir}
	require.Equal(t, dataDir+"/bouncer_key", h.bouncerKeyPath())
}

// TestBouncerKeyPathDefaultWithoutDataDirOrOverride checks the image default applies to an unset handler
// and to a nil handler.
func TestBouncerKeyPathDefaultWithoutDataDirOrOverride(t *testing.T) {
	t.Setenv(bouncerkey.PathEnvName, "")

	require.Equal(t, bouncerkey.DefaultFile, (&CrowdsecHandler{}).bouncerKeyPath())

	var nilHandler *CrowdsecHandler
	require.Equal(t, bouncerkey.DefaultFile, nilHandler.bouncerKeyPath())
}
