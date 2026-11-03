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

// Step budgets of ReplaceBouncer. A first pass is delete (5 s) then add (10 s). A failed add is retried with
// delete (5 s) then add (5 s). The four steps add up to replaceBudget, which caps the whole sequence.
// They are variables so tests can shorten them.
var (
	replaceDeleteTimeout      = 5 * time.Second
	replaceAddTimeout         = 10 * time.Second
	replaceRetryDeleteTimeout = 5 * time.Second
	replaceRetryAddTimeout    = 5 * time.Second
	replaceBudget             = 25 * time.Second
)

// ReplaceBouncer replaces the registration of the named bouncer and returns its new key. The bouncer name is the
// key's identity, so there is no rename: the old registration is deleted and a new one added.
//
// The sequence runs on a context that a cancel of ctx cannot end, so a shutdown cannot split the pair. Its total
// time is capped at replaceBudget. If the add fails, the pair is run once more (delete, then add), because the name
// may already be taken by a registration cscli committed before the failure. Only a second failure is returned.
// The returned error never contains the key or the command output.
func ReplaceBouncer(ctx context.Context, run BouncerRunner, name string) (string, error) {
	total, cancel := context.WithTimeout(context.WithoutCancel(ctx), replaceBudget)
	defer cancel()

	key, err := replaceOnce(total, run, name, replaceDeleteTimeout, replaceAddTimeout)
	if err == nil {
		return key, nil
	}
	key, retryErr := replaceOnce(total, run, name, replaceRetryDeleteTimeout, replaceRetryAddTimeout)
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
