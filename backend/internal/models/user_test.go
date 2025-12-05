package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUser_SetPassword(t *testing.T) {
	u := &User{}
	err := u.SetPassword("password123")
	assert.NoError(t, err)
	assert.NotEmpty(t, u.PasswordHash)
	assert.NotEqual(t, "password123", u.PasswordHash)
}

func TestUser_CheckPassword(t *testing.T) {
	u := &User{}
	_ = u.SetPassword("password123")

	assert.True(t, u.CheckPassword("password123"))
	assert.False(t, u.CheckPassword("wrongpassword"))
}

func TestUser_HasPendingInvite(t *testing.T) {
	tests := []struct {
		name     string
		user     User
		expected bool
	}{
		{
			name:     "no invite token",
			user:     User{InviteToken: "", InviteStatus: ""},
			expected: false,
		},
		{
			name: "expired invite",
			user: User{
				InviteToken:   "token123",
				InviteExpires: timePtr(time.Now().Add(-1 * time.Hour)),
				InviteStatus:  "pending",
			},
			expected: false,
		},
		{
			name: "valid pending invite",
			user: User{
				InviteToken:   "token123",
				InviteExpires: timePtr(time.Now().Add(24 * time.Hour)),
				InviteStatus:  "pending",
			},
			expected: true,
		},
		{
			name: "already accepted invite",
			user: User{
				InviteToken:   "token123",
				InviteExpires: timePtr(time.Now().Add(24 * time.Hour)),
				InviteStatus:  "accepted",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.user.HasPendingInvite()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUser_CanAccessHost_AllowAll(t *testing.T) {
	// User with allow_all mode (blacklist) - can access everything except listed hosts
	user := User{
		Role:           "user",
		PermissionMode: PermissionModeAllowAll,
		PermittedHosts: []ProxyHost{
			{ID: 1}, // Blocked host
			{ID: 2}, // Blocked host
		},
	}

	// Should NOT be able to access hosts in the blacklist
	assert.False(t, user.CanAccessHost(1))
	assert.False(t, user.CanAccessHost(2))

	// Should be able to access other hosts
	assert.True(t, user.CanAccessHost(3))
	assert.True(t, user.CanAccessHost(100))
}

func TestUser_CanAccessHost_DenyAll(t *testing.T) {
	// User with deny_all mode (whitelist) - can only access listed hosts
	user := User{
		Role:           "user",
		PermissionMode: PermissionModeDenyAll,
		PermittedHosts: []ProxyHost{
			{ID: 5}, // Allowed host
			{ID: 6}, // Allowed host
		},
	}

	// Should be able to access hosts in the whitelist
	assert.True(t, user.CanAccessHost(5))
	assert.True(t, user.CanAccessHost(6))

	// Should NOT be able to access other hosts
	assert.False(t, user.CanAccessHost(1))
	assert.False(t, user.CanAccessHost(100))
}

func TestUser_CanAccessHost_AdminBypass(t *testing.T) {
	// Admin users should always have access regardless of permission mode
	adminUser := User{
		Role:           "admin",
		PermissionMode: PermissionModeDenyAll,
		PermittedHosts: []ProxyHost{}, // No hosts in whitelist
	}

	// Admin should still be able to access any host
	assert.True(t, adminUser.CanAccessHost(1))
	assert.True(t, adminUser.CanAccessHost(999))
}

func TestUser_CanAccessHost_DefaultBehavior(t *testing.T) {
	// User with empty/default permission mode should behave like allow_all
	user := User{
		Role:           "user",
		PermissionMode: "", // Empty = default
		PermittedHosts: []ProxyHost{
			{ID: 1}, // Should be blocked
		},
	}

	assert.False(t, user.CanAccessHost(1))
	assert.True(t, user.CanAccessHost(2))
}

func TestUser_CanAccessHost_EmptyPermittedHosts(t *testing.T) {
	tests := []struct {
		name           string
		permissionMode PermissionMode
		hostID         uint
		expected       bool
	}{
		{
			name:           "allow_all with no exceptions allows all",
			permissionMode: PermissionModeAllowAll,
			hostID:         1,
			expected:       true,
		},
		{
			name:           "deny_all with no exceptions denies all",
			permissionMode: PermissionModeDenyAll,
			hostID:         1,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{
				Role:           "user",
				PermissionMode: tt.permissionMode,
				PermittedHosts: []ProxyHost{},
			}
			result := user.CanAccessHost(tt.hostID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPermissionMode_Constants(t *testing.T) {
	assert.Equal(t, PermissionMode("allow_all"), PermissionModeAllowAll)
	assert.Equal(t, PermissionMode("deny_all"), PermissionModeDenyAll)
}

// Helper function to create time pointers
func timePtr(t time.Time) *time.Time {
	return &t
}
