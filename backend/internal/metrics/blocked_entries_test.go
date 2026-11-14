package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecordBlockedEntry_CountsWAFAndCrowdSecOnly describes the access-log block counter: only the WAF and CrowdSec
// sources move a counter. The counters are read before and after, and the test is not parallel.
func TestRecordBlockedEntry_CountsWAFAndCrowdSecOnly(t *testing.T) {
	wafBefore := testutil.ToFloat64(WAFBlockedCounter())
	crowdsecBefore := testutil.ToFloat64(CrowdSecBlockedCounter())

	RecordBlockedEntry("waf")
	assert.Equal(t, wafBefore+1, testutil.ToFloat64(WAFBlockedCounter()))
	assert.Equal(t, crowdsecBefore, testutil.ToFloat64(CrowdSecBlockedCounter()))

	RecordBlockedEntry("crowdsec")
	assert.Equal(t, crowdsecBefore+1, testutil.ToFloat64(CrowdSecBlockedCounter()))

	RecordBlockedEntry("acl")
	RecordBlockedEntry("ratelimit")
	assert.Equal(t, wafBefore+1, testutil.ToFloat64(WAFBlockedCounter()))
	assert.Equal(t, crowdsecBefore+1, testutil.ToFloat64(CrowdSecBlockedCounter()))
}

// TestRegister_ServesEveryCollectorOnEachRegistry describes the served registry: Register works on more than one
// registry, the URL test collectors are served once recorded, and the removed monitored series is absent.
func TestRegister_ServesEveryCollectorOnEachRegistry(t *testing.T) {
	first := prometheus.NewRegistry()
	second := prometheus.NewRegistry()
	require.NotPanics(t, func() {
		Register(first)
		Register(second)
	})

	RecordURLValidation("blocked", "private_ip")
	RecordSSRFBlock("private")
	RecordURLTestDuration(0.05)

	families, err := first.Gather()
	require.NoError(t, err)
	names := make(map[string]bool, len(families))
	for _, family := range families {
		names[family.GetName()] = true
	}
	for _, want := range []string{
		"charon_url_validation_total", "charon_ssrf_blocks_total", "charon_url_test_duration_seconds",
		"charon_waf_blocked_total", "charon_crowdsec_blocked_total",
	} {
		assert.True(t, names[want], "%s is served", want)
	}
	assert.False(t, names["charon_waf_monitored_total"], "the always-zero monitored series is removed")
}
