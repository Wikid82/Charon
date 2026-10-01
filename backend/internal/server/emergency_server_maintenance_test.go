package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
)

// activeGate returns a gate in the converting phase through its public API.
func activeGate(t *testing.T) *dbmaint.Gate {
	t.Helper()
	g := dbmaint.NewGate()
	g.MarkPlanned()
	require.NoError(t, g.BeginChecking())
	require.NoError(t, g.BeginConverting(1))
	require.True(t, g.Active())
	return g
}

func startGatedEmergencyServer(t *testing.T, gate *dbmaint.Gate) (addr string, pinDB func()) {
	t.Helper()
	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-emergency-token-for-testing-32chars")
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	srv := NewEmergencyServerWithDeps(db, config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:0"}, nil, nil, gate)
	require.NoError(t, srv.Start())
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	return srv.GetAddr(), func() {
		conn, err := sqlDB.Conn(context.Background()) // pin the pool's only connection
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
	}
}

func TestEmergencyServer_ActiveGateAnswersFast503WhileHealthStays200(t *testing.T) {
	addr, pinDB := startGatedEmergencyServer(t, activeGate(t))
	pinDB()
	client := &http.Client{Timeout: time.Second}

	resp, err := client.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err, "the DB-free health must answer while the pool is pinned")
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = client.Post(fmt.Sprintf("http://%s/emergency/security-reset", addr), "application/json", http.NoBody)
	require.NoError(t, err, "a gated request must not wait for the pinned pool")
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "15", resp.Header.Get("Retry-After"))
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, true, body["maintenance"])
	assert.NotEmpty(t, body["error"])
}

func TestEmergencyServer_IdleGateServesNormally(t *testing.T) {
	addr, _ := startGatedEmergencyServer(t, dbmaint.NewGate())
	resp, err := http.Post(fmt.Sprintf("http://%s/emergency/security-reset", addr), "application/json", http.NoBody)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.NotEqual(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestEmergencyServer_NilGateServesNormally(t *testing.T) {
	addr, _ := startGatedEmergencyServer(t, nil)
	resp, err := http.Post(fmt.Sprintf("http://%s/emergency/security-reset", addr), "application/json", http.NoBody)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.NotEqual(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// The emergency server's /health is DB-free and registered before the gate, so
// it answers 200 in every phase.
func TestEmergencyServer_HealthIs200InEveryPhase(t *testing.T) {
	gates := map[string]*dbmaint.Gate{"idle": dbmaint.NewGate(), "active": activeGate(t)}
	for name, gate := range gates {
		t.Run(name, func(t *testing.T) {
			addr, _ := startGatedEmergencyServer(t, gate)
			resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}
