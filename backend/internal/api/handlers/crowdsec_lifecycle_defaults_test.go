package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lapiThenReadyCommand fails the first LAPI status check and passes every later one.
type lapiThenReadyCommand struct{ calls int }

func (c *lapiThenReadyCommand) Execute(context.Context, string, ...string) ([]byte, error) {
	c.calls++
	if c.calls == 1 {
		return nil, errors.New("lapi not ready yet")
	}
	return []byte("ok"), nil
}

// A handler with no wait bounds configured falls back to its default bounds, and the default poll interval
// still retries a failed LAPI status check until LAPI answers.
func TestStartCrowdSec_UnsetWaitBoundsUseDefaultsAndPollsUntilReady(t *testing.T) {
	db := setupCrowdDB(t)
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.CmdExec = &lapiThenReadyCommand{}
	h.LAPIMaxWait = 0
	h.LAPIPollInterval = 0

	res, err := h.StartCrowdSec(context.Background())

	require.NoError(t, err)
	assert.True(t, res.LAPIReady, "the default poll interval must retry after the first failed status check")
}
