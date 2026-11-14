package services

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestLogWatcherBroadcast_CountsBlockedEntriesBySource describes the proxied-block counters: a blocked WAF entry moves the
// WAF counter, a blocked CrowdSec entry moves the CrowdSec counter, and an entry that was not blocked moves neither. The
// counters are read before and after, and the test does not run in parallel with other counter tests.
func TestLogWatcherBroadcast_CountsBlockedEntriesBySource(t *testing.T) {
	watcher := NewLogWatcher("/tmp/unused-access.log")
	t.Cleanup(watcher.Stop)
	ch := watcher.Subscribe()
	t.Cleanup(func() { watcher.Unsubscribe(ch) })

	wafBefore := testutil.ToFloat64(metrics.WAFBlockedCounter())
	crowdsecBefore := testutil.ToFloat64(metrics.CrowdSecBlockedCounter())

	watcher.broadcast(models.SecurityLogEntry{Source: "waf", Blocked: true})
	assert.Equal(t, wafBefore+1, testutil.ToFloat64(metrics.WAFBlockedCounter()))
	assert.Equal(t, crowdsecBefore, testutil.ToFloat64(metrics.CrowdSecBlockedCounter()))

	watcher.broadcast(models.SecurityLogEntry{Source: "crowdsec", Blocked: true})
	assert.Equal(t, crowdsecBefore+1, testutil.ToFloat64(metrics.CrowdSecBlockedCounter()))
	assert.Equal(t, wafBefore+1, testutil.ToFloat64(metrics.WAFBlockedCounter()))

	watcher.broadcast(models.SecurityLogEntry{Source: "normal", Blocked: false})
	assert.Equal(t, wafBefore+1, testutil.ToFloat64(metrics.WAFBlockedCounter()))
	assert.Equal(t, crowdsecBefore+1, testutil.ToFloat64(metrics.CrowdSecBlockedCounter()))
}

// TestLogWatcherBroadcast_CountsBlocksWithoutSubscribers describes the ordering: the block is counted whether or not any
// subscriber is attached.
func TestLogWatcherBroadcast_CountsBlocksWithoutSubscribers(t *testing.T) {
	watcher := NewLogWatcher("/tmp/unused-access.log")
	t.Cleanup(watcher.Stop)

	before := testutil.ToFloat64(metrics.WAFBlockedCounter())
	watcher.broadcast(models.SecurityLogEntry{Source: "waf", Blocked: true})

	assert.Equal(t, before+1, testutil.ToFloat64(metrics.WAFBlockedCounter()))
}
