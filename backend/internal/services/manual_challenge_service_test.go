package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockDNSResolver is a mock implementation of DNSResolver for testing.
type MockDNSResolver struct {
	Records     map[string][]string
	LookupError error
}

func NewMockDNSResolver() *MockDNSResolver {
	return &MockDNSResolver{
		Records: make(map[string][]string),
	}
}

func (m *MockDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if m.LookupError != nil {
		return nil, m.LookupError
	}
	if records, ok := m.Records[name]; ok {
		return records, nil
	}
	return nil, errors.New("no such host")
}

func (m *MockDNSResolver) SetRecords(fqdn string, values []string) {
	m.Records[fqdn] = values
}

func setupManualChallengeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.ManualChallenge{})
	require.NoError(t, err)

	return db
}

func TestNewManualChallengeService(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	require.NotNil(t, service)
	assert.NotNil(t, service.db)
	assert.NotNil(t, service.cron)
	assert.NotNil(t, service.resolver)
}

func TestManualChallengeService_SetResolver(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	mockResolver := NewMockDNSResolver()
	service.SetResolver(mockResolver)

	assert.Equal(t, mockResolver, service.resolver)
}

func TestManualChallengeService_CreateChallenge(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Token:      "token123",
		Value:      "txtvalue456",
	}

	challenge, err := service.CreateChallenge(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, challenge)
	assert.NotEmpty(t, challenge.ID)
	assert.Equal(t, uint(1), challenge.ProviderID)
	assert.Equal(t, uint(1), challenge.UserID)
	assert.Equal(t, "_acme-challenge.example.com", challenge.FQDN)
	assert.Equal(t, "token123", challenge.Token)
	assert.Equal(t, "txtvalue456", challenge.Value)
	assert.Equal(t, models.ChallengeStatusPending, challenge.Status)
	assert.False(t, challenge.DNSPropagated)
	assert.True(t, challenge.ExpiresAt.After(time.Now()))
}

func TestManualChallengeService_CreateChallenge_SameUserReturnsExisting(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Token:      "token123",
		Value:      "txtvalue456",
	}

	// Create first challenge
	challenge1, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// Try to create another for same FQDN and user
	challenge2, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// Should return the same challenge
	assert.Equal(t, challenge1.ID, challenge2.ID)
}

func TestManualChallengeService_GetChallenge(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Token:      "token123",
		Value:      "txtvalue456",
	}

	created, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// Get the challenge
	challenge, err := service.GetChallenge(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, challenge.ID)
}

func TestManualChallengeService_GetChallenge_NotFound(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	_, err := service.GetChallenge(ctx, "nonexistent-id")

	assert.ErrorIs(t, err, ErrChallengeNotFound)
}

func TestManualChallengeService_GetChallengeForUser(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Token:      "token123",
		Value:      "txtvalue456",
	}

	created, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// Same user should be able to access
	challenge, err := service.GetChallengeForUser(ctx, created.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, created.ID, challenge.ID)

	// Different user should get unauthorized error
	_, err = service.GetChallengeForUser(ctx, created.ID, 2)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestManualChallengeService_ListChallengesForProvider(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()

	// Create challenges for provider 1
	for i := 0; i < 3; i++ {
		_, err := service.CreateChallenge(ctx, CreateChallengeRequest{
			ProviderID: 1,
			UserID:     1,
			FQDN:       "_acme-challenge" + string(rune('a'+i)) + ".example.com",
			Value:      "value" + string(rune('a'+i)),
		})
		require.NoError(t, err)
	}

	// Create challenge for different provider
	_, err := service.CreateChallenge(ctx, CreateChallengeRequest{
		ProviderID: 2,
		UserID:     1,
		FQDN:       "_acme-challenge.other.com",
		Value:      "other",
	})
	require.NoError(t, err)

	// List challenges for provider 1
	challenges, err := service.ListChallengesForProvider(ctx, 1, 1)
	require.NoError(t, err)
	assert.Len(t, challenges, 3)

	// List challenges for provider 2
	challenges, err = service.ListChallengesForProvider(ctx, 2, 1)
	require.NoError(t, err)
	assert.Len(t, challenges, 1)
}

func TestManualChallengeService_VerifyChallenge_DNSFound(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	mockResolver := NewMockDNSResolver()
	service.SetResolver(mockResolver)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Token:      "token123",
		Value:      "txtvalue456",
	}

	challenge, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// Set up mock DNS to return the expected value
	mockResolver.SetRecords("_acme-challenge.example.com", []string{"txtvalue456"})

	// Verify the challenge
	result, err := service.VerifyChallenge(ctx, challenge.ID, 1)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.True(t, result.DNSFound)
	assert.Equal(t, "verified", result.Status)
}

func TestManualChallengeService_VerifyChallenge_DNSNotFound(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	mockResolver := NewMockDNSResolver()
	service.SetResolver(mockResolver)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Token:      "token123",
		Value:      "txtvalue456",
	}

	challenge, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// DNS not set up - should not find record
	result, err := service.VerifyChallenge(ctx, challenge.ID, 1)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.False(t, result.DNSFound)
	assert.Equal(t, "pending", result.Status)
}

func TestManualChallengeService_VerifyChallenge_Expired(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()

	// Create an expired challenge directly
	challenge := &models.ManualChallenge{
		ID:         "expired-challenge",
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Value:      "value",
		Status:     models.ChallengeStatusPending,
		CreatedAt:  time.Now().Add(-20 * time.Minute),
		ExpiresAt:  time.Now().Add(-10 * time.Minute), // Already expired
	}
	err := db.Create(challenge).Error
	require.NoError(t, err)

	// Verify should fail with expired error
	_, err = service.VerifyChallenge(ctx, challenge.ID, 1)
	assert.ErrorIs(t, err, ErrChallengeExpired)
}

func TestManualChallengeService_VerifyChallenge_AlreadyVerified(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()

	// Create a verified challenge directly
	now := time.Now()
	challenge := &models.ManualChallenge{
		ID:            "verified-challenge",
		ProviderID:    1,
		UserID:        1,
		FQDN:          "_acme-challenge.example.com",
		Value:         "value",
		Status:        models.ChallengeStatusVerified,
		DNSPropagated: true,
		CreatedAt:     now.Add(-5 * time.Minute),
		ExpiresAt:     now.Add(5 * time.Minute),
		VerifiedAt:    &now,
	}
	err := db.Create(challenge).Error
	require.NoError(t, err)

	// Verify should return success
	result, err := service.VerifyChallenge(ctx, challenge.ID, 1)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "verified", result.Status)
}

func TestManualChallengeService_PollChallengeStatus(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Value:      "txtvalue456",
	}

	challenge, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	status, err := service.PollChallengeStatus(ctx, challenge.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, challenge.ID, status.ID)
	assert.Equal(t, "pending", status.Status)
	assert.False(t, status.DNSPropagated)
	assert.Greater(t, status.TimeRemainingSeconds, 0)
}

func TestManualChallengeService_DeleteChallenge(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Value:      "txtvalue456",
	}

	challenge, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// Delete the challenge
	err = service.DeleteChallenge(ctx, challenge.ID, 1)
	require.NoError(t, err)

	// Should not be found anymore
	_, err = service.GetChallenge(ctx, challenge.ID)
	assert.ErrorIs(t, err, ErrChallengeNotFound)
}

func TestManualChallengeService_DeleteChallenge_Unauthorized(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Value:      "txtvalue456",
	}

	challenge, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// Try to delete as different user
	err = service.DeleteChallenge(ctx, challenge.ID, 2)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestManualChallengeService_GetActiveChallengeForFQDN(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()
	fqdn := "_acme-challenge.example.com"

	// No active challenge yet
	challenge, err := service.GetActiveChallengeForFQDN(ctx, fqdn, 1)
	require.NoError(t, err)
	assert.Nil(t, challenge)

	// Create a challenge
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       fqdn,
		Value:      "txtvalue456",
	}
	created, err := service.CreateChallenge(ctx, req)
	require.NoError(t, err)

	// Now should find active challenge
	challenge, err = service.GetActiveChallengeForFQDN(ctx, fqdn, 1)
	require.NoError(t, err)
	require.NotNil(t, challenge)
	assert.Equal(t, created.ID, challenge.ID)
}

func TestManualChallengeService_checkDNSPropagation(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	mockResolver := NewMockDNSResolver()
	service.SetResolver(mockResolver)

	ctx := context.Background()
	fqdn := "_acme-challenge.example.com"
	expectedValue := "txtvalue456"

	// No record - should return false
	found := service.checkDNSPropagation(ctx, fqdn, expectedValue)
	assert.False(t, found)

	// Add wrong value - should return false
	mockResolver.SetRecords(fqdn, []string{"wrongvalue"})
	found = service.checkDNSPropagation(ctx, fqdn, expectedValue)
	assert.False(t, found)

	// Add correct value - should return true
	mockResolver.SetRecords(fqdn, []string{expectedValue})
	found = service.checkDNSPropagation(ctx, fqdn, expectedValue)
	assert.True(t, found)

	// Multiple records including correct value - should return true
	mockResolver.SetRecords(fqdn, []string{"other", expectedValue, "another"})
	found = service.checkDNSPropagation(ctx, fqdn, expectedValue)
	assert.True(t, found)
}

func TestManualChallengeService_checkDNSPropagation_Error(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	mockResolver := NewMockDNSResolver()
	mockResolver.LookupError = errors.New("DNS lookup failed")
	service.SetResolver(mockResolver)

	ctx := context.Background()

	found := service.checkDNSPropagation(ctx, "_acme-challenge.example.com", "value")
	assert.False(t, found)
}

func TestManualChallengeService_StartStop(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	// Start should not panic
	service.Start()

	// Stop should not panic
	service.Stop()
}

func TestDefaultDNSResolver_LookupTXT(t *testing.T) {
	resolver := NewDefaultDNSResolver()
	require.NotNil(t, resolver)
	require.NotNil(t, resolver.resolver)

	// This test may fail depending on network, so we just verify the resolver is created correctly
	ctx := context.Background()

	// Try to lookup a known domain (may fail in CI due to network)
	_, err := resolver.LookupTXT(ctx, "google.com")
	// We don't assert on the result since it depends on network
	_ = err
}

func TestChallengeStatusResponse_Fields(t *testing.T) {
	now := time.Now()
	resp := &ChallengeStatusResponse{
		ID:                   "test-id",
		Status:               "pending",
		DNSPropagated:        false,
		TimeRemainingSeconds: 300,
		LastCheckAt:          &now,
	}

	assert.Equal(t, "test-id", resp.ID)
	assert.Equal(t, "pending", resp.Status)
	assert.False(t, resp.DNSPropagated)
	assert.Equal(t, 300, resp.TimeRemainingSeconds)
	assert.NotNil(t, resp.LastCheckAt)
}

func TestVerifyResult_Fields(t *testing.T) {
	result := &VerifyResult{
		Success:       true,
		DNSFound:      true,
		Message:       "DNS TXT record verified successfully",
		Status:        "verified",
	}

	assert.True(t, result.Success)
	assert.True(t, result.DNSFound)
	assert.Equal(t, "DNS TXT record verified successfully", result.Message)
	assert.Equal(t, "verified", result.Status)
}

func TestCreateChallengeRequest_Fields(t *testing.T) {
	req := CreateChallengeRequest{
		ProviderID: 1,
		UserID:     2,
		FQDN:       "_acme-challenge.example.com",
		Token:      "token",
		Value:      "value",
	}

	assert.Equal(t, uint(1), req.ProviderID)
	assert.Equal(t, uint(2), req.UserID)
	assert.Equal(t, "_acme-challenge.example.com", req.FQDN)
	assert.Equal(t, "token", req.Token)
	assert.Equal(t, "value", req.Value)
}

func TestManualChallengeService_CleanupExpiredChallenges(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	// Create challenges with different states and expiration times
	now := time.Now()

	// Create an expired pending challenge
	expiredChallenge := &models.ManualChallenge{
		ID:         "expired-pending",
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.expired.com",
		Value:      "value1",
		Status:     models.ChallengeStatusPending,
		CreatedAt:  now.Add(-20 * time.Minute),
		ExpiresAt:  now.Add(-10 * time.Minute),
	}
	require.NoError(t, db.Create(expiredChallenge).Error)

	// Create an old challenge (should be deleted)
	oldChallenge := &models.ManualChallenge{
		ID:         "old-challenge",
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.old.com",
		Value:      "value2",
		Status:     models.ChallengeStatusVerified,
		CreatedAt:  now.Add(-8 * 24 * time.Hour), // 8 days old
		ExpiresAt:  now.Add(-8 * 24 * time.Hour),
	}
	require.NoError(t, db.Create(oldChallenge).Error)

	// Create an active challenge (should not be affected)
	activeChallenge := &models.ManualChallenge{
		ID:         "active-challenge",
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.active.com",
		Value:      "value3",
		Status:     models.ChallengeStatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(10 * time.Minute),
	}
	require.NoError(t, db.Create(activeChallenge).Error)

	// Run cleanup
	service.cleanupExpiredChallenges()

	// Verify expired challenge is marked as expired
	var expiredResult models.ManualChallenge
	db.Where("id = ?", "expired-pending").First(&expiredResult)
	assert.Equal(t, models.ChallengeStatusExpired, expiredResult.Status)

	// Verify old challenge is deleted
	var oldCount int64
	db.Model(&models.ManualChallenge{}).Where("id = ?", "old-challenge").Count(&oldCount)
	assert.Equal(t, int64(0), oldCount)

	// Verify active challenge is unchanged
	var activeResult models.ManualChallenge
	db.Where("id = ?", "active-challenge").First(&activeResult)
	assert.Equal(t, models.ChallengeStatusPending, activeResult.Status)
}

func TestManualChallengeService_PollChallengeStatus_Expired(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()

	// Create an expired but not yet marked challenge
	now := time.Now()
	challenge := &models.ManualChallenge{
		ID:         "poll-expired",
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.expired.com",
		Value:      "value",
		Status:     models.ChallengeStatusPending,
		CreatedAt:  now.Add(-20 * time.Minute),
		ExpiresAt:  now.Add(-5 * time.Minute), // Already expired
	}
	require.NoError(t, db.Create(challenge).Error)

	// Poll should mark it as expired
	status, err := service.PollChallengeStatus(ctx, challenge.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, "expired", status.Status)
}

func TestManualChallengeService_CreateChallenge_DifferentUser(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()

	// Create a challenge for user 1
	_, err := service.CreateChallenge(ctx, CreateChallengeRequest{
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Value:      "value1",
	})
	require.NoError(t, err)

	// Try to create another for same FQDN but different user
	_, err = service.CreateChallenge(ctx, CreateChallengeRequest{
		ProviderID: 1,
		UserID:     2,
		FQDN:       "_acme-challenge.example.com",
		Value:      "value2",
	})
	assert.ErrorIs(t, err, ErrChallengeInProgress)
}

func TestManualChallengeService_ListChallengesForProvider_Empty(t *testing.T) {
	db := setupManualChallengeTestDB(t)
	service := NewManualChallengeService(db)

	ctx := context.Background()

	challenges, err := service.ListChallengesForProvider(ctx, 999, 1)
	require.NoError(t, err)
	assert.Empty(t, challenges)
}
