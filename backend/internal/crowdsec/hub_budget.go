package crowdsec

import (
	"context"
	"time"
)

// minHubAttemptBudget is the least time one hub request attempt receives, so a short remaining budget still
// leaves a real attempt. The parent deadline still bounds every attempt.
const minHubAttemptBudget = 5 * time.Second

// attemptContext returns the context for the next of attemptsLeft hub attempts made under parent. The attempt
// gets its share of the time left before parent's deadline (remaining/attemptsLeft), never less than
// minHubAttemptBudget and never more than what remains. A non-positive attemptsLeft counts as one. A parent
// without a deadline is returned unchanged with a no-op cancel: the caller that owns the overall deadline sets
// it before calling.
func attemptContext(parent context.Context, attemptsLeft int) (context.Context, context.CancelFunc) {
	deadline, ok := parent.Deadline()
	if !ok {
		return parent, func() {}
	}
	if attemptsLeft < 1 {
		attemptsLeft = 1
	}
	remaining := time.Until(deadline)
	budget := remaining / time.Duration(attemptsLeft)
	if budget < minHubAttemptBudget {
		budget = minHubAttemptBudget
	}
	if budget > remaining {
		budget = remaining
	}
	return context.WithTimeout(parent, budget)
}
