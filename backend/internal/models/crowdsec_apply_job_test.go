package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrowdsecApplyJobJSONTagsAreSnakeCase(t *testing.T) {
	total := 3
	raw, err := json.Marshal(CrowdsecApplyJob{ID: "x", ItemsTotal: &total, ErrorCode: "e", Error: "m", Warnings: "[]", BackupPath: "/b"})
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	for _, k := range []string{"id", "slug", "item_type", "cache_key", "state", "phase", "error_code", "error", "warnings",
		"backup_path", "items_installed", "items_total", "cancel_requested", "created_at", "started_at", "finished_at", "updated_at"} {
		require.Contains(t, m, k)
	}
}

func TestCrowdsecApplyJobActiveStates(t *testing.T) {
	for _, s := range []string{CrowdsecApplyJobQueued, CrowdsecApplyJobRunning, CrowdsecApplyJobVerifying} {
		require.True(t, (&CrowdsecApplyJob{State: s}).IsActive(), s)
	}
	for _, s := range []string{CrowdsecApplyJobSucceeded, CrowdsecApplyJobFailed, CrowdsecApplyJobTimedOut, CrowdsecApplyJobCancelled, ""} {
		require.False(t, (&CrowdsecApplyJob{State: s}).IsActive(), s)
	}
	require.Equal(t, "interrupted", CrowdsecApplyErrInterrupted)
}
