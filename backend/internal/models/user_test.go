package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestUser_SetPassword(t *testing.T) {
	u := &User{}
	err := u.SetPassword("password123")
	assert.NoError(t, err)
	assert.NotEmpty(t, u.PasswordHash)
	assert.NotEqual(t, "password123", u.PasswordHash)

	// Test with empty password (should still work but hash empty string)
	u2 := &User{}
	err = u2.SetPassword("")
	assert.NoError(t, err)
	assert.NotEmpty(t, u2.PasswordHash)

	// Test with special characters
	u3 := &User{}
	err = u3.SetPassword("P@ssw0rd!#$%^&*()")
	assert.NoError(t, err)
	assert.NotEmpty(t, u3.PasswordHash)
	assert.True(t, u3.CheckPassword("P@ssw0rd!#$%^&*()"))
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
		Role:           RoleUser,
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
		Role:           RoleUser,
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
		Role:           RoleAdmin,
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
		Role:           RoleUser,
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
				Role:           RoleUser,
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

func TestUserRole_Constants(t *testing.T) {
	assert.Equal(t, UserRole("admin"), RoleAdmin)
	assert.Equal(t, UserRole("user"), RoleUser)
	assert.Equal(t, UserRole("passthrough"), RolePassthrough)
}

func TestUserRole_IsValid(t *testing.T) {
	tests := []struct {
		role     UserRole
		expected bool
	}{
		{RoleAdmin, true},
		{RoleUser, true},
		{RolePassthrough, true},
		{UserRole("viewer"), false},
		{UserRole("superadmin"), false},
		{UserRole(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.role.IsValid())
		})
	}
}

// Helper function to create time pointers
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestHashPasswordCost(t *testing.T) {
	hash, err := HashPasswordCost("password123", bcrypt.MinCost)
	assert.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("password123")))

	_, err = HashPasswordCost("password123", bcrypt.MaxCost+1)
	assert.Error(t, err)
}

func TestHashPassword_DefaultCost(t *testing.T) {
	hash, err := HashPassword("password123")
	assert.NoError(t, err)
	cost, err := bcrypt.Cost([]byte(hash))
	assert.NoError(t, err)
	assert.Equal(t, bcrypt.DefaultCost, cost)
}

func TestUser_InviteStateAt(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Minute)

	tests := []struct {
		name string
		user User
		want InviteState
	}{
		{name: "no token", user: User{InviteStatus: InviteStatusPending, InviteExpires: &future}, want: InviteNone},
		{name: "pending with future expiry", user: User{InviteToken: "t", InviteStatus: InviteStatusPending, InviteExpires: &future}, want: InviteOpen},
		{name: "pending with expiry in the past", user: User{InviteToken: "t", InviteStatus: InviteStatusPending, InviteExpires: &past}, want: InviteExpired},
		{name: "pending with expiry equal to now", user: User{InviteToken: "t", InviteStatus: InviteStatusPending, InviteExpires: &now}, want: InviteExpired},
		{name: "pending with expiry unset fails closed", user: User{InviteToken: "t", InviteStatus: InviteStatusPending}, want: InviteExpired},
		{name: "accepted", user: User{InviteToken: "t", InviteStatus: InviteStatusAccepted, InviteExpires: &future}, want: InviteClosed},
		{name: "accepted with expiry unset stays closed", user: User{InviteToken: "t", InviteStatus: InviteStatusAccepted}, want: InviteClosed},
		{name: "expired status with passed expiry reports expired", user: User{InviteToken: "t", InviteStatus: InviteStatusExpired, InviteExpires: &past}, want: InviteExpired},
		{name: "expired status with future expiry", user: User{InviteToken: "t", InviteStatus: InviteStatusExpired, InviteExpires: &future}, want: InviteClosed},
		{name: "empty status", user: User{InviteToken: "t", InviteExpires: &future}, want: InviteClosed},
		{name: "pending but password already set", user: User{InviteToken: "t", InviteStatus: InviteStatusPending, InviteExpires: &future, PasswordHash: "hash"}, want: InviteClosed},
		{name: "legacy enabled pending invite still open", user: User{InviteToken: "t", InviteStatus: InviteStatusPending, InviteExpires: &future, Enabled: true}, want: InviteOpen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.user.InviteStateAt(now))
		})
	}
}

func TestUser_HasPendingInvite_RefusesPasswordSet(t *testing.T) {
	future := time.Now().Add(time.Hour)
	withPassword := &User{InviteToken: "t", InviteStatus: InviteStatusPending, InviteExpires: &future, PasswordHash: "hash"}
	assert.False(t, withPassword.HasPendingInvite())
}
