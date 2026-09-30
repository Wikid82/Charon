package util

import (
	"context"
	"time"
)

// SleepContext waits for d or until ctx is cancelled, returning ctx's error in
// the latter case so shutdown is never held up by a pause between work chunks.
func SleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
