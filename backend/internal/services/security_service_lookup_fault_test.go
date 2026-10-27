package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// A store failure during a lookup is reported as a lookup error, not as a missing config.
func TestSecurityService_GetByNameReportsStoreFailure(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	gormfault.Inject(t, db, gormfault.Query, "security_configs", nil)

	cfg, err := svc.GetByName("default")

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.NotErrorIs(t, err, ErrSecurityConfigNotFound)
	assert.ErrorContains(t, err, "find security config")
}
