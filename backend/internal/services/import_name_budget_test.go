package services

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestImportNameBudget_CheckedWhileWithinBudget(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, map[string][]string{"imds.import.example": {"169.254.169.254"}}))
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)

	budget := NewImportNameBudget()
	defer budget.Close()
	host := &models.ProxyHost{DomainNames: "imds.example.com", ForwardHost: "imds.import.example", ForwardPort: 80}
	assert.ErrorIs(t, svc.CreateImported(budget, host), ErrForwardTargetAddress, "a name inside the budget is checked")
	assert.Equal(t, 0, budget.uncheckedHosts)
}

func TestImportNameBudget_SpentBudgetAcceptsNamesUnchecked(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, map[string][]string{"late.import.example": {"169.254.169.254"}}))
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)

	budget := NewImportNameBudget()
	budget.cancel()
	host := &models.ProxyHost{DomainNames: "late.example.com", ForwardHost: "late.import.example", ForwardPort: 80}
	require.NoError(t, svc.CreateImported(budget, host), "a name past the budget is accepted unchecked")
	assert.Zero(t, calls.Load(), "no lookup runs after the budget is spent")
	assert.Equal(t, 1, budget.uncheckedHosts)

	literal := &models.ProxyHost{DomainNames: "lit.example.com", ForwardHost: "169.254.169.254", ForwardPort: 80}
	assert.ErrorIs(t, svc.CreateImported(budget, literal), ErrForwardTargetAddress, "literal addresses are still checked")
}

func TestImportNameBudget_LookupCutByBudgetCountsOnceAndIsNotCached(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)

	budget := NewImportNameBudget()
	defer budget.Close()
	stubLookup(t, func(context.Context, string) ([]net.IPAddr, error) {
		budget.cancel() // the budget runs out during this lookup
		return nil, errors.New("lookup cut short")
	})
	host := &models.ProxyHost{DomainNames: "cut.example.com", ForwardHost: "cut.import.example", ForwardPort: 80}
	require.NoError(t, svc.CreateImported(budget, host))
	assert.Equal(t, 1, budget.uncheckedHosts)
	assert.Empty(t, forwardNameCache.entries, "a lookup cut short by the budget is not cached")
}

func TestImportNameBudget_CloseWarnsOnceWithCountOnly(t *testing.T) {
	hook := logtest.NewLocal(logger.Log().Logger)
	defer hook.Reset()

	clean := NewImportNameBudget()
	clean.Close()
	assert.Empty(t, hook.AllEntries(), "no warning when every name was checked")

	skipped := NewImportNameBudget()
	skipped.uncheckedHosts = 3
	skipped.Close()
	require.Len(t, hook.AllEntries(), 1)
	entry := hook.LastEntry()
	assert.Equal(t, 3, entry.Data["unchecked_hosts"])
	for key := range entry.Data {
		assert.NotContains(t, []string{"host", "address", "name", "domain"}, key, "no host or address is logged")
	}
}

func TestImportNameBudget_NilCloseIsSafe(t *testing.T) {
	t.Parallel()
	var budget *ImportNameBudget
	budget.Close()
}
