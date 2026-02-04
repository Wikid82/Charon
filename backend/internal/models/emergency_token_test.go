package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEmergencyToken_TableName(t *testing.T) {
	token := EmergencyToken{}
	assert.Equal(t, "emergency_tokens", token.TableName())
}

func TestEmergencyToken_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  bool
	}{
		{
			name:      "nil expiration (never expires)",
			expiresAt: nil,
			expected:  false,
		},
		{
			name:      "expired token (1 hour ago)",
			expiresAt: ptrTime(now.Add(-1 * time.Hour)),
			expected:  true,
		},
		{
			name:      "expired token (1 day ago)",
			expiresAt: ptrTime(now.Add(-24 * time.Hour)),
			expected:  true,
		},
		{
			name:      "valid token (1 hour from now)",
			expiresAt: ptrTime(now.Add(1 * time.Hour)),
			expected:  false,
		},
		{
			name:      "valid token (30 days from now)",
			expiresAt: ptrTime(now.Add(30 * 24 * time.Hour)),
			expected:  false,
		},
		{
			name:      "expired by 1 second",
			expiresAt: ptrTime(now.Add(-1 * time.Second)),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &EmergencyToken{
				ExpiresAt: tt.expiresAt,
			}
			result := token.IsExpired()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEmergencyToken_DaysUntilExpiration(t *testing.T) {
	// Test with actual time.Now() since the method uses it internally
	now := time.Now()

	tests := []struct {
		name    string
		expires *time.Time
		minDays int
		maxDays int
	}{
		{
			name:    "nil expiration",
			expires: nil,
			minDays: -1,
			maxDays: -1,
		},
		{
			name:    "expires in ~1 day",
			expires: ptrTime(now.Add(24 * time.Hour)),
			minDays: 0,
			maxDays: 1,
		},
		{
			name:    "expires in ~30 days",
			expires: ptrTime(now.Add(30 * 24 * time.Hour)),
			minDays: 29,
			maxDays: 30,
		},
		{
			name:    "expires in ~60 days",
			expires: ptrTime(now.Add(60 * 24 * time.Hour)),
			minDays: 59,
			maxDays: 60,
		},
		{
			name:    "expires in ~90 days",
			expires: ptrTime(now.Add(90 * 24 * time.Hour)),
			minDays: 89,
			maxDays: 90,
		},
		{
			name:    "expired ~1 day ago",
			expires: ptrTime(now.Add(-24 * time.Hour)),
			minDays: -2,
			maxDays: -1,
		},
		{
			name:    "expired ~10 days ago",
			expires: ptrTime(now.Add(-10 * 24 * time.Hour)),
			minDays: -11,
			maxDays: -10,
		},
		{
			name:    "expires in ~12 hours (partial day)",
			expires: ptrTime(now.Add(12 * time.Hour)),
			minDays: 0,
			maxDays: 1,
		},
		{
			name:    "expires in ~36 hours (1.5 days)",
			expires: ptrTime(now.Add(36 * time.Hour)),
			minDays: 1,
			maxDays: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &EmergencyToken{ExpiresAt: tt.expires}
			result := token.DaysUntilExpiration()

			assert.GreaterOrEqual(t, result, tt.minDays, "days should be >= min")
			assert.LessOrEqual(t, result, tt.maxDays, "days should be <= max")
		})
	}
}

// ptrTime is a helper to create a pointer to a time.Time
func ptrTime(t time.Time) *time.Time {
	return &t
}
