package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/util"
)

// BouncerRunner runs one cscli command. The step context already carries the step's deadline; timeout is the
// same step budget, for runners that also apply it themselves.
type BouncerRunner func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error)

// ReplaceTimings are the step budgets of ReplaceBouncerWith. A first pass is delete then add; a failed add is
// retried with delete then add again. The four steps add up to Total, which caps the whole sequence. A zero field
// takes its default (see DefaultReplaceTimings), so a caller can shorten only the budgets it needs.
type ReplaceTimings struct {
	Delete      time.Duration
	Add         time.Duration
	RetryDelete time.Duration
	RetryAdd    time.Duration
	Total       time.Duration
}

// DefaultReplaceTimings returns the budgets of ReplaceBouncer: delete 5 s, add 10 s, retry delete 5 s, retry add
// 5 s, and a total of 25 s.
func DefaultReplaceTimings() ReplaceTimings {
	return ReplaceTimings{
		Delete:      5 * time.Second,
		Add:         10 * time.Second,
		RetryDelete: 5 * time.Second,
		RetryAdd:    5 * time.Second,
		Total:       25 * time.Second,
	}
}

// withDefaults fills every zero field of t from DefaultReplaceTimings.
func (t ReplaceTimings) withDefaults() ReplaceTimings {
	def := DefaultReplaceTimings()
	if t.Delete == 0 {
		t.Delete = def.Delete
	}
	if t.Add == 0 {
		t.Add = def.Add
	}
	if t.RetryDelete == 0 {
		t.RetryDelete = def.RetryDelete
	}
	if t.RetryAdd == 0 {
		t.RetryAdd = def.RetryAdd
	}
	if t.Total == 0 {
		t.Total = def.Total
	}
	return t
}

// ReplaceBouncer replaces the registration of the named bouncer and returns its new key, with the default budgets
// (see ReplaceBouncerWith).
func ReplaceBouncer(ctx context.Context, run BouncerRunner, name string) (string, error) {
	return ReplaceBouncerWith(ctx, run, name, ReplaceTimings{})
}

// ReplaceBouncerWith replaces the registration of the named bouncer and returns its new key. The bouncer name is the
// key's identity, so there is no rename: the old registration is deleted and a new one added.
//
// The sequence runs on a context that a cancel of ctx cannot end, so a shutdown cannot split the pair. Its total
// time is capped at timings.Total. If the add fails, the pair is run once more (delete, then add), because the name
// may already be taken by a registration cscli committed before the failure. Only a second failure is returned.
// The returned error never contains the key or the command output.
func ReplaceBouncerWith(ctx context.Context, run BouncerRunner, name string, timings ReplaceTimings) (string, error) {
	t := timings.withDefaults()
	total, cancel := context.WithTimeout(context.WithoutCancel(ctx), t.Total)
	defer cancel()

	key, err := replaceOnce(total, run, name, t.Delete, t.Add)
	if err == nil {
		return key, nil
	}
	key, retryErr := replaceOnce(total, run, name, t.RetryDelete, t.RetryAdd)
	if retryErr != nil {
		return "", fmt.Errorf("bouncer registration failed after a retry: %w", retryErr)
	}
	return key, nil
}

// replaceOnce runs one delete then add pass. The delete result is ignored: "not found" is the expected answer
// when there is no stale registration. The add must print exactly the key.
func replaceOnce(ctx context.Context, run BouncerRunner, name string, deleteStep, addStep time.Duration) (string, error) {
	deleteCtx, cancelDelete := context.WithTimeout(ctx, deleteStep)
	_, _ = run(deleteCtx, deleteStep, "bouncers", "delete", name)
	cancelDelete()

	addCtx, cancelAdd := context.WithTimeout(ctx, addStep)
	defer cancelAdd()
	output, err := run(addCtx, addStep, "bouncers", "add", name, "-o", "raw")
	if err != nil {
		return "", fmt.Errorf("bouncer registration failed: %w (%s)", err, util.DescribeCommandOutput(output))
	}
	key := strings.TrimSpace(string(output))
	if key == "" {
		return "", errors.New("bouncer registration returned empty key")
	}
	return key, nil
}
