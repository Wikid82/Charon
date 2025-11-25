package models

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthPolicyTestDB(t *testing.T) *gorm.DB {
	dsn := filepath.Join(t.TempDir(), "test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuthPolicy{}))
	return db
}

func TestAuthPolicy_BeforeCreate(t *testing.T) {
	db := setupAuthPolicyTestDB(t)

	t.Run("generates UUID when empty", func(t *testing.T) {
		policy := &AuthPolicy{
			Name: "test-policy",
		}
		require.NoError(t, db.Create(policy).Error)
		assert.NotEmpty(t, policy.UUID)
		assert.Len(t, policy.UUID, 36)
	})

	t.Run("keeps existing UUID", func(t *testing.T) {
		customUUID := "custom-policy-uuid"
		policy := &AuthPolicy{
			UUID: customUUID,
			Name: "test-policy-2",
		}
		require.NoError(t, db.Create(policy).Error)
		assert.Equal(t, customUUID, policy.UUID)
	})
}

func TestAuthPolicy_IsPublic(t *testing.T) {
	tests := []struct {
		name     string
		policy   AuthPolicy
		expected bool
	}{
		{
			name:     "empty restrictions is public",
			policy:   AuthPolicy{},
			expected: true,
		},
		{
			name: "only roles set is not public",
			policy: AuthPolicy{
				AllowedRoles: "admin",
			},
			expected: false,
		},
		{
			name: "only users set is not public",
			policy: AuthPolicy{
				AllowedUsers: "user@example.com",
			},
			expected: false,
		},
		{
			name: "only domains set is not public",
			policy: AuthPolicy{
				AllowedDomains: "@example.com",
			},
			expected: false,
		},
		{
			name: "all restrictions set is not public",
			policy: AuthPolicy{
				AllowedRoles:   "admin",
				AllowedUsers:   "user@example.com",
				AllowedDomains: "@example.com",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.policy.IsPublic())
		})
	}
}
