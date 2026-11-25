package upstreamdns

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// defaultResolvTimeout is the glibc and Go default when resolv.conf sets none.
	defaultResolvTimeout = 5 * time.Second
	minLookupTimeout     = 500 * time.Millisecond
	maxLookupTimeout     = 3 * time.Second
	// lookupTimeoutMargin keeps the answerer's failure ahead of the client's own
	// timeout, so the client gets SERVFAIL instead of giving up.
	lookupTimeoutMargin = 500 * time.Millisecond
)

// LookupTimeoutFromResolvConf returns the per-lookup timeout for the resolv.conf at
// path: its options timeout minus a margin, clamped to 500 ms - 3 s. A missing file or
// a missing option uses the default of 5 s.
func LookupTimeoutFromResolvConf(path string) time.Duration {
	timeout := defaultResolvTimeout
	if data, err := os.ReadFile(filepath.Clean(path)); err == nil {
		if parsed, ok := parseResolvTimeout(data); ok {
			timeout = parsed
		}
	}
	return min(max(timeout-lookupTimeoutMargin, minLookupTimeout), maxLookupTimeout)
}

// parseResolvTimeout reads "timeout:N" from an options line.
func parseResolvTimeout(data []byte) (time.Duration, bool) {
	for _, line := range bytes.Split(data, []byte("\n")) {
		fields := strings.Fields(string(line))
		if len(fields) == 0 || fields[0] != "options" {
			continue
		}
		for _, opt := range fields[1:] {
			raw, found := strings.CutPrefix(opt, "timeout:")
			if !found {
				continue
			}
			seconds, err := strconv.Atoi(raw)
			if err != nil || seconds < 0 {
				return 0, false
			}
			return time.Duration(seconds) * time.Second, true
		}
	}
	return 0, false
}
