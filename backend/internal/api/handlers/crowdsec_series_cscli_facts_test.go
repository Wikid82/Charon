package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Facts recorded from the cscli bundled with the image (testdata/cscli/v1.8.2-rc1): which alerts a time filter
// keeps, that there is no offset flag, and the text of a failed hub download with the network cut.

func recordedAlertScenarios(t *testing.T, fixture string) []string {
	t.Helper()
	var alerts []struct {
		Scenario string `json:"scenario"`
	}
	require.NoError(t, json.Unmarshal(cscliFixture(t, fixture), &alerts), fixture)
	out := make([]string, 0, len(alerts))
	for _, a := range alerts {
		out = append(out, a.Scenario)
	}
	return out
}

func TestCSCLIContract_AlertsTimeFilters_RecordedFacts(t *testing.T) {
	t.Parallel()
	const newest, oldest = "manual/new-alert", "manual/old-alert"
	// The recorded alerts started now and three days ago. --since compares the start of the alert.
	cases := map[string][]string{
		"alerts_list_all_json":              {newest, oldest},
		"alerts_list_limit_0_json":          {newest, oldest},
		"alerts_list_limit_1_json":          {newest},
		"alerts_list_since_1h_json":         {newest},
		"alerts_list_since_24h_json":        {newest},
		"alerts_list_since_7d_json":         {newest, oldest},
		"alerts_list_since_30d_limit0_json": {newest, oldest},
		"alerts_list_until_1h_json":         {oldest},
		"alerts_list_until_24h_json":        {oldest},
	}
	for fixture, want := range cases {
		assert.Equal(t, want, recordedAlertScenarios(t, fixture), fixture)
		assert.Zero(t, cscliFixtureExit(t, fixture), fixture)
	}
}

func TestCSCLIContract_AlertsList_NoOffsetFlag(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 1, cscliFixtureExit(t, "alerts_list_offset_json"))
	assert.Contains(t, string(cscliFixture(t, "alerts_list_offset_json")), "unknown flag: --offset")
	assert.Equal(t, 1, cscliFixtureExit(t, "alerts_list_since_badunit_json"))
	assert.Contains(t, string(cscliFixture(t, "alerts_list_since_badunit_json")), `unknown unit "w"`)
}

// With the network cut, `hub update` and every typed install fail with a network error and exit 1. The markers
// below are what Charon matches to tell an unreachable hub from any other install failure.
func TestCSCLIContract_OfflineHubErrors_RecordedFacts(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"offline", "internal"} {
		for _, cmd := range []string{"hub_update", "collections_install", "parsers_install", "appsec_rules_install"} {
			name := cmd + "_" + kind
			text := string(cscliFixture(t, name))
			assert.Equal(t, 1, cscliFixtureExit(t, name), name)
			for _, marker := range []string{"dial tcp", "lookup"} {
				assert.Contains(t, text, marker, name)
			}
			assert.Contains(t, text, "failed http request", name)
			if kind == "offline" {
				assert.Contains(t, text, "network is unreachable", name)
			} else {
				assert.Contains(t, text, "server misbehaving", name)
			}
			assert.False(t, strings.Contains(text, "can't find"), name)
		}
	}
}
