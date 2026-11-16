package services

import (
	"context"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// importNameBudget bounds the name lookups of one import. It is shared by every
// host the import writes, so a slow resolver cannot stretch an import without limit.
const importNameBudget = 30 * time.Second

// ImportNameBudget is the name-lookup budget of one import. Once the budget is spent,
// the names of the remaining hosts are accepted without a check, and the import
// reports one count when it closes. No host, name or address is recorded.
type ImportNameBudget struct {
	ctx            context.Context
	cancel         context.CancelFunc
	uncheckedHosts int
}

// NewImportNameBudget starts the budget for one import. Call Close when the import ends.
func NewImportNameBudget() *ImportNameBudget {
	ctx, cancel := context.WithTimeout(context.Background(), importNameBudget)
	return &ImportNameBudget{ctx: ctx, cancel: cancel}
}

// Close ends the budget and, when some names went unchecked, logs one warning with the count.
func (b *ImportNameBudget) Close() {
	if b == nil {
		return
	}
	b.cancel()
	if b.uncheckedHosts > 0 {
		logger.Log().WithField("unchecked_hosts", b.uncheckedHosts).
			Warn("Import: name checks were skipped after the import time budget ran out")
	}
}

// preflight collects the name verdicts of one host within the import budget. Names
// whose lookup did not finish in time are dropped from the verdicts, so the host
// is checked on its literal addresses only.
func (b *ImportNameBudget) preflight(host *models.ProxyHost) forwardVerdicts {
	if b.ctx.Err() != nil {
		b.uncheckedHosts++
		return forwardVerdicts{policy: probePolicy(), names: map[string]nameVerdict{}}
	}
	v := preflightForwardNames(b.ctx, forwardNamesOf(host))
	if b.ctx.Err() != nil {
		unresolved := false
		for name, nv := range v.names {
			if nv.err != nil {
				delete(v.names, name)
				unresolved = true
			}
		}
		if unresolved {
			b.uncheckedHosts++
		}
	}
	return v
}

// CreateImported is CreateAs for an import host, with the import's name budget.
func (s *ProxyHostService) CreateImported(b *ImportNameBudget, host *models.ProxyHost) error {
	return s.createChecked(host, true, b.preflight(host))
}

// UpdateImported is UpdateAs for an import host, with the import's name budget.
func (s *ProxyHostService) UpdateImported(b *ImportNameBudget, host *models.ProxyHost) error {
	return s.updateChecked(host, true, b.preflight(host))
}
