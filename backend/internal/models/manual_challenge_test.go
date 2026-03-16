package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChallengeStatus_Constants(t *testing.T) {
	// Verify all expected status values exist
	assert.Equal(t, ChallengeStatus("created"), ChallengeStatusCreated)
	assert.Equal(t, ChallengeStatus("pending"), ChallengeStatusPending)
	assert.Equal(t, ChallengeStatus("verifying"), ChallengeStatusVerifying)
	assert.Equal(t, ChallengeStatus("verified"), ChallengeStatusVerified)
	assert.Equal(t, ChallengeStatus("expired"), ChallengeStatusExpired)
	assert.Equal(t, ChallengeStatus("failed"), ChallengeStatusFailed)
}

func TestManualChallenge_TableName(t *testing.T) {
	challenge := ManualChallenge{}
	assert.Equal(t, "manual_challenges", challenge.TableName())
}

func TestManualChallenge_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   ChallengeStatus
		expected bool
	}{
		{"created is not terminal", ChallengeStatusCreated, false},
		{"pending is not terminal", ChallengeStatusPending, false},
		{"verifying is not terminal", ChallengeStatusVerifying, false},
		{"verified is terminal", ChallengeStatusVerified, true},
		{"expired is terminal", ChallengeStatusExpired, true},
		{"failed is terminal", ChallengeStatusFailed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenge := &ManualChallenge{Status: tt.status}
			assert.Equal(t, tt.expected, challenge.IsTerminal())
		})
	}
}

func TestManualChallenge_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   ChallengeStatus
		expected bool
	}{
		{"created is active", ChallengeStatusCreated, true},
		{"pending is active", ChallengeStatusPending, true},
		{"verifying is active", ChallengeStatusVerifying, true},
		{"verified is not active", ChallengeStatusVerified, false},
		{"expired is not active", ChallengeStatusExpired, false},
		{"failed is not active", ChallengeStatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenge := &ManualChallenge{Status: tt.status}
			assert.Equal(t, tt.expected, challenge.IsActive())
		})
	}
}

func TestManualChallenge_TimeRemaining(t *testing.T) {
	tests := []struct {
		name           string
		expiresAt      time.Time
		expectPositive bool
	}{
		{
			name:           "future expiration returns positive duration",
			expiresAt:      time.Now().Add(10 * time.Minute),
			expectPositive: true,
		},
		{
			name:           "past expiration returns zero",
			expiresAt:      time.Now().Add(-5 * time.Minute),
			expectPositive: false,
		},
		{
			name:           "just expired returns zero",
			expiresAt:      time.Now().Add(-1 * time.Second),
			expectPositive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenge := &ManualChallenge{ExpiresAt: tt.expiresAt}
			remaining := challenge.TimeRemaining()

			if tt.expectPositive {
				assert.Greater(t, remaining, time.Duration(0))
			} else {
				assert.Equal(t, time.Duration(0), remaining)
			}
		})
	}
}

func TestManualChallenge_StructFields(t *testing.T) {
	now := time.Now()
	lastCheck := now.Add(-1 * time.Minute)
	verified := now.Add(-30 * time.Second)

	challenge := &ManualChallenge{
		ID:            "test-uuid-123",
		ProviderID:    1,
		UserID:        2,
		FQDN:          "_acme-challenge.example.com",
		Token:         "token123",
		Value:         "txtvalue456",
		Status:        ChallengeStatusPending,
		ErrorMessage:  "",
		DNSPropagated: false,
		CreatedAt:     now,
		ExpiresAt:     now.Add(10 * time.Minute),
		LastCheckAt:   &lastCheck,
		VerifiedAt:    &verified,
	}

	assert.Equal(t, "test-uuid-123", challenge.ID)
	assert.Equal(t, uint(1), challenge.ProviderID)
	assert.Equal(t, uint(2), challenge.UserID)
	assert.Equal(t, "_acme-challenge.example.com", challenge.FQDN)
	assert.Equal(t, "token123", challenge.Token)
	assert.Equal(t, "txtvalue456", challenge.Value)
	assert.Equal(t, ChallengeStatusPending, challenge.Status)
	assert.Empty(t, challenge.ErrorMessage)
	assert.False(t, challenge.DNSPropagated)
	assert.Equal(t, now, challenge.CreatedAt)
	assert.Equal(t, now.Add(10*time.Minute), challenge.ExpiresAt)
	assert.NotNil(t, challenge.LastCheckAt)
	assert.NotNil(t, challenge.VerifiedAt)
}

func TestManualChallenge_NilOptionalFields(t *testing.T) {
	challenge := &ManualChallenge{
		ID:          "test-uuid",
		ProviderID:  1,
		UserID:      1,
		FQDN:        "_acme-challenge.example.com",
		Value:       "value",
		Status:      ChallengeStatusCreated,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(10 * time.Minute),
		LastCheckAt: nil,
		VerifiedAt:  nil,
	}

	assert.Nil(t, challenge.LastCheckAt)
	assert.Nil(t, challenge.VerifiedAt)
	assert.True(t, challenge.IsActive())
	assert.False(t, challenge.IsTerminal())
}
