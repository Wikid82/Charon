package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// An imported host carries the same advanced configuration limit as a host the user saves.
func TestImportedHostRefusesAnAdvancedConfigOverTheLimit(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	budget := NewImportNameBudget()
	defer budget.Close()

	oversized := `{"pad":"` + strings.Repeat("a", MaxAdvancedConfigBytes) + `"}`
	host := &models.ProxyHost{DomainNames: "oversized-import.example.com", ForwardHost: "10.0.0.7", ForwardPort: 8080, AdvancedConfig: oversized}

	require.ErrorIs(t, svc.CreateImported(budget, host), ErrAdvancedConfigTooLarge)

	var count int64
	require.NoError(t, db.Model(&models.ProxyHost{}).Where("domain_names = ?", "oversized-import.example.com").Count(&count).Error)
	assert.Zero(t, count, "the refused import is not stored")
}

func TestImportedHostAcceptsAnAdvancedConfigAtTheLimit(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	budget := NewImportNameBudget()
	defer budget.Close()

	// The value is the object wrapper plus padding, sized so the normalised JSON is exactly at the limit.
	const wrapper = `{"pad":""}`
	atLimit := `{"pad":"` + strings.Repeat("a", MaxAdvancedConfigBytes-len(wrapper)) + `"}`
	require.Len(t, atLimit, MaxAdvancedConfigBytes)
	host := &models.ProxyHost{DomainNames: "limit-import.example.com", ForwardHost: "10.0.0.8", ForwardPort: 8080, AdvancedConfig: atLimit}

	require.NoError(t, svc.CreateImported(budget, host))
}
