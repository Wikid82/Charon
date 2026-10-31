package crowdsec

import (
	"context"
	"time"
)

// cscliListReserve is the time kept back from the index walk for the local `cscli hub list` that FetchIndex runs
// when no hub address answers. The reserve applies only when the walk has more than two attempt floors of time.
const cscliListReserve = 2 * time.Second

// pullBudget returns total, or the default pull timeout when total is not set (a HubService built without one).
func pullBudget(total time.Duration) time.Duration {
	if total <= 0 {
		return defaultPullTimeout
	}
	return total
}

// overallContext returns the deadline that bounds one index fetch. A caller that set a deadline keeps its own;
// otherwise the fetch gets total from now. The caller must call cancel.
func overallContext(parent context.Context, total time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := parent.Deadline(); ok {
		return parent, func() {}
	}
	return context.WithTimeout(parent, pullBudget(total))
}

// walkContext returns the deadline of the HTTP candidate walk inside parent: the time left on parent (total when
// parent has no deadline), less the cscli listing reserve when there is room for it. The caller must call cancel.
func walkContext(parent context.Context, total time.Duration) (context.Context, context.CancelFunc) {
	remaining := pullBudget(total)
	if deadline, ok := parent.Deadline(); ok {
		remaining = time.Until(deadline)
	}
	if remaining > 2*minHubAttemptBudget {
		remaining -= cscliListReserve
	}
	return context.WithTimeout(parent, remaining)
}
