package ratelimit

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// WarnBudget caps how many WARN lines externally triggered events may emit.
// Events over the cap are counted, and the count is handed to the next event
// that may log, so operators still see the volume without a log flood.
type WarnBudget struct {
	mu         sync.Mutex
	lim        *rate.Limiter
	now        func() time.Time
	suppressed uint64
}

// NewWarnBudget allows up to n WARNs per interval (burst n). A nil clock uses
// time.Now; non-positive inputs are clamped to one WARN per second.
func NewWarnBudget(n int, interval time.Duration, now func() time.Time) *WarnBudget {
	n = max(n, 1)
	if interval <= 0 {
		interval = time.Second
	}
	if now == nil {
		now = time.Now
	}
	r, b := PerWindow(n, interval)
	return &WarnBudget{lim: rate.NewLimiter(r, b), now: now}
}

// Take reports whether a WARN may be emitted now. When it may, it also returns
// the number of events suppressed since the last emitted WARN and resets it.
func (w *WarnBudget) Take() (bool, uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.lim.AllowN(w.now(), 1) {
		w.suppressed++
		return false, 0
	}
	suppressed := w.suppressed
	w.suppressed = 0
	return true, suppressed
}

// LogDenial logs a throttle denial on entry, adding retry_after_seconds. The
// first denial of an episode is logged at WARN while the budget allows (with a
// suppressed count); every other denial is logged at DEBUG.
func (w *WarnBudget) LogDenial(entry *logrus.Entry, d Decision, msg string) {
	entry = entry.WithField("retry_after_seconds", RetryAfterSeconds(d.RetryAfter))
	if d.FirstDenial {
		if ok, suppressed := w.Take(); ok {
			entry.WithField("suppressed", suppressed).Warn(msg)
			return
		}
	}
	entry.Debug(msg)
}
