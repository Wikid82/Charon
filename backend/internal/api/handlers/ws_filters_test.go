package handlers

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTrackedFilters_KeepsOnlyAllowedKeys verifies that only allow-listed query keys reach the connection list.
func TestTrackedFilters_KeepsOnlyAllowedKeys(t *testing.T) {
	t.Parallel()

	values, err := url.ParseQuery("source=waf&token=tkn-test-value&level=error&ip=10.0.0.1")
	assert.NoError(t, err)

	got := trackedFilters(values, []string{"source", "level", "ip", "host", "blocked_only"})

	assert.Equal(t, "ip=10.0.0.1&level=error&source=waf", got)
	assert.NotContains(t, got, "tkn-test-value")
}

// TestTrackedFilters_EmptyWhenNoAllowedKeysPresent verifies an empty result for unrelated query keys.
func TestTrackedFilters_EmptyWhenNoAllowedKeysPresent(t *testing.T) {
	t.Parallel()

	values, err := url.ParseQuery("token=tkn-test-value")
	assert.NoError(t, err)

	assert.Empty(t, trackedFilters(values, []string{"source", "level"}))
	assert.Empty(t, trackedFilters(url.Values{}, []string{"source"}))
}
