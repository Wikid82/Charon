package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
)

// scriptedApply returns the errors in order, one per call, and records each call's context.
type scriptedApply struct {
	errs    []error
	seen    []context.Context
	seenErr []error // ctx.Err() at the time of each call
}

func (s *scriptedApply) ApplyConfig(ctx context.Context) error {
	s.seen = append(s.seen, ctx)
	s.seenErr = append(s.seenErr, ctx.Err())
	if len(s.errs) == 0 {
		return nil
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	return err
}

func TestApplyOrRestoreWithoutAManagerAppliesNothing(t *testing.T) {
	t.Parallel()
	restored := false
	err := applyOrRestore(context.Background(), nil, func() error {
		restored = true
		return nil
	})
	require.NoError(t, err)
	assert.False(t, restored)
}

func TestApplyOrRestoreKeepsTheWriteWhenTheApplyWorks(t *testing.T) {
	t.Parallel()
	applier := &scriptedApply{}
	restored := false
	err := applyOrRestore(context.Background(), applier, func() error {
		restored = true
		return nil
	})
	require.NoError(t, err)
	assert.False(t, restored, "a successful apply keeps the write")
	assert.Len(t, applier.seen, 1)
}

func TestApplyOrRestoreRestoresAndReturnsTheApplyError(t *testing.T) {
	t.Parallel()
	applyErr := errors.New("unknown handler module")
	applier := &scriptedApply{errs: []error{applyErr}}
	restored := 0
	err := applyOrRestore(context.Background(), applier, func() error {
		restored++
		return nil
	})
	require.ErrorIs(t, err, applyErr)
	assert.Equal(t, 1, restored)
	assert.Len(t, applier.seen, 1, "no re-apply without a failed rollback")
}

func TestApplyOrRestoreLogsARestoreFailureAndStillReturnsTheApplyError(t *testing.T) {
	t.Parallel()
	applyErr := errors.New("refused")
	applier := &scriptedApply{errs: []error{applyErr}}
	restoreErr := errors.New("database is locked")
	err := applyOrRestore(context.Background(), applier, func() error { return restoreErr })
	require.ErrorIs(t, err, applyErr)
	assert.NotErrorIs(t, err, restoreErr, "the restore failure is logged, not returned")
}

func TestApplyOrRestoreReappliesOnceWhenTheRollbackAlsoFailed(t *testing.T) {
	t.Parallel()
	rollbackErr := fmt.Errorf("apply failed: %w, %w: %v", errRefused, caddy.ErrRollbackFailed, "timeout")
	applier := &scriptedApply{errs: []error{rollbackErr, nil}}
	restored := 0
	err := applyOrRestore(context.Background(), applier, func() error {
		restored++
		return nil
	})
	require.ErrorIs(t, err, caddy.ErrRollbackFailed)
	assert.Equal(t, 1, restored)
	assert.Len(t, applier.seen, 2, "one best-effort re-apply after the restore")
}

func TestApplyOrRestoreKeepsTheApplyErrorWhenTheReapplyAlsoFails(t *testing.T) {
	t.Parallel()
	rollbackErr := fmt.Errorf("apply failed: %w, %w: %v", errRefused, caddy.ErrRollbackFailed, "timeout")
	applier := &scriptedApply{errs: []error{rollbackErr, errors.New("still down")}}
	err := applyOrRestore(context.Background(), applier, func() error { return nil })
	require.ErrorIs(t, err, caddy.ErrRollbackFailed)
	assert.Len(t, applier.seen, 2)
}

func TestApplyOrRestoreRunsTheApplyOnAContextThatCannotBeCancelled(t *testing.T) {
	t.Parallel()
	applier := &scriptedApply{errs: []error{errRefused}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = applyOrRestore(ctx, applier, func() error { return nil })

	require.Len(t, applier.seen, 1)
	assert.NoError(t, applier.seenErr[0], "a client that left cannot cancel the apply or the restore")
	_, hasDeadline := applier.seen[0].Deadline()
	assert.True(t, hasDeadline, "the apply is bounded")
}

func TestRespondProxyApplyFailedAnswersAdministratorsWithTheDetail(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/x", nil)
	c.Set(middleware.RoleKey, string(models.RoleAdmin))

	respondProxyApplyFailed(c, errors.New("unknown module \"x\"\nline two"), gin.H{"updated": 0})

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "previous settings kept): unknown module")
	assert.Contains(t, w.Body.String(), `"error_code":"proxy_apply_failed"`)
	assert.Contains(t, w.Body.String(), `"updated":0`)
}

func TestRespondProxyApplyFailedGivesOtherCallersTheFixedTextOnly(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/x", nil)
	c.Set(middleware.RoleKey, string(models.RoleUser))

	respondProxyApplyFailed(c, errors.New("internal detail"), nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to apply configuration (previous settings kept)")
	assert.NotContains(t, w.Body.String(), "internal detail")
	assert.Contains(t, w.Body.String(), `"error_code":"proxy_apply_failed"`)
}

func TestProxyHostLeftOutHostsIsNilWithoutAManager(t *testing.T) {
	t.Parallel()
	h := &ProxyHostHandler{}
	assert.Nil(t, h.leftOutHosts())
}
