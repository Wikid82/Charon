package crowdsec

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const replaceTestKey = "replacement-key-0123456789abcdef"

// replayCall is one recorded runner call.
type replayCall struct {
	args        []string
	timeout     time.Duration
	deadline    time.Time
	hasDeadline bool
	ctxErr      error
}

// addResult is the answer to one "bouncers add" call.
type addResult struct {
	out []byte
	err error
}

// replayRunner records every call. Adds are answered from adds in order; once it is empty every add returns
// replaceTestKey. onDelete runs after a delete. hangAdd makes an add wait for its context and return its error.
type replayRunner struct {
	mu       sync.Mutex
	calls    []replayCall
	adds     []addResult
	onDelete func()
	hangAdd  bool
}

func (r *replayRunner) run(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	dl, hasDeadline := ctx.Deadline()
	r.mu.Lock()
	r.calls = append(r.calls, replayCall{args: args, timeout: timeout, deadline: dl, hasDeadline: hasDeadline, ctxErr: ctx.Err()})
	isAdd := len(args) > 1 && args[0] == "bouncers" && args[1] == "add"
	var next addResult
	hasNext := false
	if isAdd && len(r.adds) > 0 {
		next, hasNext = r.adds[0], true
		r.adds = r.adds[1:]
	}
	hang, onDelete := r.hangAdd, r.onDelete
	r.mu.Unlock()

	if isAdd {
		if hang {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		if hasNext {
			return next.out, next.err
		}
		return []byte(replaceTestKey + "\n"), nil
	}
	if onDelete != nil {
		onDelete()
	}
	return nil, nil
}

func (r *replayRunner) lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.calls))
	for _, c := range r.calls {
		out = append(out, strings.Join(c.args, " "))
	}
	return out
}

func (r *replayRunner) snapshot() []replayCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]replayCall(nil), r.calls...)
}

// shortTimings returns the four step budgets and the total, for tests that must not wait the real 25 s.
func shortTimings(total, deleteStep, addStep, retryDeleteStep, retryAddStep time.Duration) ReplaceTimings {
	return ReplaceTimings{Delete: deleteStep, Add: addStep, RetryDelete: retryDeleteStep, RetryAdd: retryAddStep, Total: total}
}

func TestReplaceBouncer_BudgetMatchesTheStepSum(t *testing.T) {
	def := DefaultReplaceTimings()
	assert.Equal(t, 25*time.Second, def.Total)
	assert.Equal(t, def.Total, def.Delete+def.Add+def.RetryDelete+def.RetryAdd)
}

func TestReplaceBouncerWith_ZeroFieldsTakeTheDefaults(t *testing.T) {
	assert.Equal(t, DefaultReplaceTimings(), ReplaceTimings{}.withDefaults())

	partial := ReplaceTimings{Add: time.Second}.withDefaults()
	assert.Equal(t, time.Second, partial.Add, "a set field is kept")
	assert.Equal(t, DefaultReplaceTimings().Delete, partial.Delete, "an unset field takes its default")
}

func TestReplaceBouncer_DeletesThenAddsAndReturnsTrimmedKey(t *testing.T) {
	r := &replayRunner{adds: []addResult{{out: []byte("  " + replaceTestKey + "\n")}}}

	key, err := ReplaceBouncer(context.Background(), r.run, "caddy-bouncer")

	require.NoError(t, err)
	assert.Equal(t, replaceTestKey, key)
	assert.Equal(t, []string{"bouncers delete caddy-bouncer", "bouncers add caddy-bouncer -o raw"}, r.lines())
}

func TestReplaceBouncer_CancelledBeforeTheCallStillCompletesBothSteps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &replayRunner{}

	key, err := ReplaceBouncer(ctx, r.run, "caddy-bouncer")

	require.NoError(t, err)
	assert.Equal(t, replaceTestKey, key)
	calls := r.snapshot()
	require.Len(t, calls, 2)
	for i, c := range calls {
		assert.NoError(t, c.ctxErr, "call %d runs on a context the parent cancel cannot end", i)
	}
}

func TestReplaceBouncer_CancelBetweenTheStepsDoesNotSplitThePair(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &replayRunner{onDelete: cancel}

	key, err := ReplaceBouncer(ctx, r.run, "caddy-bouncer")

	require.NoError(t, err)
	assert.Equal(t, replaceTestKey, key)
	assert.Equal(t, []string{"bouncers delete caddy-bouncer", "bouncers add caddy-bouncer -o raw"}, r.lines())
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "the parent was cancelled between the steps")
	calls := r.snapshot()
	require.Len(t, calls, 2)
	assert.NoError(t, calls[1].ctxErr, "the add runs on a context the cancel cannot end")
}

func TestReplaceBouncer_FailedAddIsRetriedOnceWithDeleteAndAdd(t *testing.T) {
	r := &replayRunner{adds: []addResult{{err: errors.New("exit status 1")}}}

	key, err := ReplaceBouncer(context.Background(), r.run, "caddy-bouncer")

	require.NoError(t, err)
	assert.Equal(t, replaceTestKey, key)
	assert.Equal(t, []string{
		"bouncers delete caddy-bouncer",
		"bouncers add caddy-bouncer -o raw",
		"bouncers delete caddy-bouncer",
		"bouncers add caddy-bouncer -o raw",
	}, r.lines())
}

func TestReplaceBouncer_SecondAddFailureReturnsAnErrorWithoutOutput(t *testing.T) {
	const sentinel = "secret-looking-output-0123456789"
	r := &replayRunner{adds: []addResult{
		{out: []byte(sentinel), err: errors.New("exit status 1")},
		{out: []byte(sentinel), err: errors.New("exit status 1")},
	}}

	key, err := ReplaceBouncer(context.Background(), r.run, "caddy-bouncer")

	require.Error(t, err)
	assert.Empty(t, key)
	assert.ErrorContains(t, err, "exit status 1")
	assert.ErrorContains(t, err, "bytes of output omitted")
	assert.NotContains(t, err.Error(), sentinel)
	assert.Len(t, r.lines(), 4)
}

func TestReplaceBouncer_EmptyKeyIsAFailureOnBothPasses(t *testing.T) {
	r := &replayRunner{adds: []addResult{{out: []byte("  \n")}, {out: []byte(" \n")}}}

	key, err := ReplaceBouncer(context.Background(), r.run, "caddy-bouncer")

	require.Error(t, err)
	assert.Empty(t, key)
	assert.ErrorContains(t, err, "empty key")
	assert.Len(t, r.lines(), 4)
}

func TestReplaceBouncer_StepDeadlinesAreTheStatedValues(t *testing.T) {
	r := &replayRunner{adds: []addResult{{err: errors.New("exit status 1")}}}
	def := DefaultReplaceTimings()

	_, err := ReplaceBouncer(context.Background(), r.run, "caddy-bouncer")

	require.NoError(t, err)
	calls := r.snapshot()
	require.Len(t, calls, 4)
	want := []time.Duration{def.Delete, def.Add, def.RetryDelete, def.RetryAdd}
	for i, c := range calls {
		require.True(t, c.hasDeadline, "call %d has a deadline", i)
		assert.Equal(t, want[i], c.timeout, "call %d timeout argument", i)
		remaining := time.Until(c.deadline)
		assert.LessOrEqual(t, remaining, want[i], "call %d deadline", i)
		assert.Greater(t, remaining, want[i]-time.Second, "call %d deadline", i)
	}
}

func TestReplaceBouncer_HungAddIsCutAtItsStepTimeoutAndRetried(t *testing.T) {
	r := &replayRunner{hangAdd: true}

	_, err := ReplaceBouncerWith(context.Background(), r.run, "caddy-bouncer", shortTimings(5*time.Second, 5*time.Second, 50*time.Millisecond, 5*time.Second, 30*time.Millisecond))

	require.Error(t, err)
	calls := r.snapshot()
	require.Len(t, calls, 4)
	assert.Equal(t, 50*time.Millisecond, calls[1].timeout)
	assert.Equal(t, 30*time.Millisecond, calls[3].timeout)
}

func TestReplaceBouncer_TotalBudgetCapsTheWholeSequence(t *testing.T) {
	r := &replayRunner{hangAdd: true}

	start := time.Now()
	_, err := ReplaceBouncerWith(context.Background(), r.run, "caddy-bouncer", shortTimings(150*time.Millisecond, 2*time.Second, 2*time.Second, 2*time.Second, 2*time.Second))

	require.Error(t, err)
	assert.Less(t, time.Since(start), 1500*time.Millisecond, "the budget ends the sequence, not the per-step caps")
}

// TestReplaceBouncer_BothAddsHangStaysWithinTheTotal runs both passes with every add hung. Each add waits for its
// own context, so the elapsed time is set by the budgets alone, and every call's deadline falls inside the total.
func TestReplaceBouncer_BothAddsHangStaysWithinTheTotal(t *testing.T) {
	const total = 100 * time.Millisecond
	const slack = 500 * time.Millisecond
	r := &replayRunner{hangAdd: true}

	start := time.Now()
	_, err := ReplaceBouncerWith(context.Background(), r.run, "caddy-bouncer", shortTimings(total, 20*time.Millisecond, 60*time.Millisecond, 20*time.Millisecond, 60*time.Millisecond))

	require.Error(t, err)
	assert.Less(t, time.Since(start), total+slack, "the sequence ends within its total budget")
	calls := r.snapshot()
	require.Len(t, calls, 4)
	for i, c := range calls {
		require.True(t, c.hasDeadline, "call %d has a deadline", i)
		assert.LessOrEqual(t, c.deadline.Sub(start), total+slack, "call %d deadline is inside the total", i)
	}
}
