package handlers

import (
	"net/url"
)

// trackedFilters returns the subset of query values whose keys are in allowed, re-encoded in sorted order.
// Connection info is exposed through the management API, so only known filter keys may be recorded; any other
// query parameter (for example an authentication token) is dropped.
func trackedFilters(values url.Values, allowed []string) string {
	kept := make(url.Values, len(allowed))
	for _, key := range allowed {
		if vals, ok := values[key]; ok {
			kept[key] = vals
		}
	}
	return kept.Encode()
}

// cerberusTrackedFilterKeys are the query keys recorded for Cerberus live-log connections.
var cerberusTrackedFilterKeys = []string{"source", "level", "ip", "host", "blocked_only"}

// logsTrackedFilterKeys are the query keys recorded for log socket connections.
var logsTrackedFilterKeys = []string{"level", "source"}
