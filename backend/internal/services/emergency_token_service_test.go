package services

import (
	"crypto/sha256"
	"os"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupEmergencyTokenTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.EmergencyToken{})
	require.NoError(t, err)

	return db
}

func TestEmergencyTokenService_Generate(t *testing.T) {
	tests := []struct {
		name           string
		expirationDays int
		expectedPolicy string
	}{
		{
			name:           "30 days policy",
			expirationDays: 30,
			expectedPolicy: "30_days",
		},
		{
			name:           "60 days policy",
			expirationDays: 60,
			expectedPolicy: "60_days",
		},
		{
			name:           "90 days policy",
			expirationDays: 90,
			expectedPolicy: "90_days",
		},
		{
			name:           "custom 45 days policy",
			expirationDays: 45,
			expectedPolicy: "custom_45_days",
		},
		{
			name:           "never expires",
			expirationDays: 0,
			expectedPolicy: "never",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupEmergencyTokenTestDB(t)
			svc := NewEmergencyTokenService(db)

			userID := uint(1)
			resp, err := svc.Generate(GenerateRequest{
				ExpirationDays: tt.expirationDays,
				UserID:         &userID,
			})

			require.NoError(t, err)
			assert.NotEmpty(t, resp.Token)
			assert.Equal(t, tt.expectedPolicy, resp.ExpirationPolicy)

			// Token should be 128 hex characters (64 bytes)
			assert.Len(t, resp.Token, 128)

			// Verify expiration
			if tt.expirationDays > 0 {
				assert.NotNil(t, resp.ExpiresAt)
				expectedExpiry := time.Now().Add(time.Duration(tt.expirationDays) * 24 * time.Hour)
				assert.WithinDuration(t, expectedExpiry, *resp.ExpiresAt, time.Minute)
			} else {
				assert.Nil(t, resp.ExpiresAt)
			}

			// Verify database record
			var tokenRecord models.EmergencyToken
			err = db.First(&tokenRecord).Error
			require.NoError(t, err)
			assert.Equal(t, tt.expectedPolicy, tokenRecord.ExpirationPolicy)

			// Verify bcrypt hash (not plaintext)
			tokenHash := sha256.Sum256([]byte(resp.Token))
			err = bcrypt.CompareHashAndPassword([]byte(tokenRecord.TokenHash), tokenHash[:])
			assert.NoError(t, err, "Token should be stored as bcrypt hash")
		})
	}
}

func TestEmergencyTokenService_Generate_ReplacesOldToken(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Generate first token
	resp1, err := svc.Generate(GenerateRequest{ExpirationDays: 90})
	require.NoError(t, err)

	// Generate second token
	resp2, err := svc.Generate(GenerateRequest{ExpirationDays: 60})
	require.NoError(t, err)

	// Verify tokens are different
	assert.NotEqual(t, resp1.Token, resp2.Token)

	// Verify only one token in database
	var count int64
	db.Model(&models.EmergencyToken{}).Count(&count)
	assert.Equal(t, int64(1), count)

	// Verify old token no longer validates
	_, err = svc.Validate(resp1.Token)
	assert.Error(t, err)

	// Verify new token validates
	_, err = svc.Validate(resp2.Token)
	assert.NoError(t, err)
}

func TestEmergencyTokenService_Validate(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Generate token
	resp, err := svc.Generate(GenerateRequest{ExpirationDays: 90})
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid token",
			token:       resp.Token,
			expectError: false,
		},
		{
			name:        "invalid token",
			token:       "invalid-token-12345",
			expectError: true,
			errorMsg:    "invalid token",
		},
		{
			name:        "empty token",
			token:       "",
			expectError: true,
			errorMsg:    "token is empty",
		},
		{
			name:        "whitespace token",
			token:       "   ",
			expectError: true,
			errorMsg:    "token is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenRecord, err := svc.Validate(tt.token)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, tokenRecord)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tokenRecord)
				assert.Greater(t, tokenRecord.UseCount, 0)
				assert.NotNil(t, tokenRecord.LastUsedAt)
			}
		})
	}
}

func TestEmergencyTokenService_Validate_Expiration(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Generate token with short expiration
	resp, err := svc.Generate(GenerateRequest{ExpirationDays: 1})
	require.NoError(t, err)

	// Manually expire the token
	var tokenRecord models.EmergencyToken
	db.First(&tokenRecord)
	past := time.Now().Add(-25 * time.Hour)
	tokenRecord.ExpiresAt = &past
	db.Save(&tokenRecord)

	// Validate should fail
	_, err = svc.Validate(resp.Token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestEmergencyTokenService_Validate_EnvironmentFallback(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Set environment variable
	envToken := "this-is-a-long-test-token-for-environment-fallback-validation"
	_ = os.Setenv(EmergencyTokenEnvVar, envToken)
	defer func() { _ = os.Unsetenv(EmergencyTokenEnvVar) }()

	// Validate with environment token (no DB token exists)
	tokenRecord, err := svc.Validate(envToken)
	assert.NoError(t, err)
	assert.Nil(t, tokenRecord, "Env var tokens return nil record")
}

func TestEmergencyTokenService_Validate_EnvironmentBreakGlassFallback(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Set environment variable
	envToken := "this-is-a-long-test-token-for-environment-fallback-validation"
	_ = os.Setenv(EmergencyTokenEnvVar, envToken)
	defer func() { _ = os.Unsetenv(EmergencyTokenEnvVar) }()

	// Generate database token
	dbResp, err := svc.Generate(GenerateRequest{ExpirationDays: 90})
	require.NoError(t, err)

	// Database token should validate
	_, err = svc.Validate(dbResp.Token)
	assert.NoError(t, err)

	// Environment token should still validate as break-glass fallback
	_, err = svc.Validate(envToken)
	assert.NoError(t, err)
}

func TestEmergencyTokenService_GetStatus(t *testing.T) {
	t.Run("no token configured", func(t *testing.T) {
		db := setupEmergencyTokenTestDB(t)
		svc := NewEmergencyTokenService(db)

		status, err := svc.GetStatus()
		require.NoError(t, err)

		assert.False(t, status.Configured)
		assert.Equal(t, "none", status.Source)
		assert.Nil(t, status.CreatedAt)
		assert.Nil(t, status.ExpiresAt)
	})

	t.Run("database token configured", func(t *testing.T) {
		db := setupEmergencyTokenTestDB(t)
		svc := NewEmergencyTokenService(db)

		// Generate token
		resp, err := svc.Generate(GenerateRequest{ExpirationDays: 90})
		require.NoError(t, err)

		// Get status
		status, err := svc.GetStatus()
		require.NoError(t, err)

		assert.True(t, status.Configured)
		assert.Equal(t, "database", status.Source)
		assert.NotNil(t, status.CreatedAt)
		assert.NotNil(t, status.ExpiresAt)
		assert.Equal(t, "90_days", status.ExpirationPolicy)
		assert.False(t, status.IsExpired)
		assert.Greater(t, status.DaysUntilExpiration, 85)

		// Validate token to update usage
		_, err = svc.Validate(resp.Token)
		require.NoError(t, err)

		// Check updated status
		status, err = svc.GetStatus()
		require.NoError(t, err)
		assert.Equal(t, 1, status.UseCount)
		assert.NotNil(t, status.LastUsedAt)
	})

	t.Run("environment token configured", func(t *testing.T) {
		db := setupEmergencyTokenTestDB(t)
		svc := NewEmergencyTokenService(db)

		// Set environment variable
		envToken := "this-is-a-long-test-token-for-environment-configuration"
		_ = os.Setenv(EmergencyTokenEnvVar, envToken)
		defer func() { _ = os.Unsetenv(EmergencyTokenEnvVar) }()

		// Get status
		status, err := svc.GetStatus()
		require.NoError(t, err)

		assert.True(t, status.Configured)
		assert.Equal(t, "environment", status.Source)
		assert.Equal(t, "never", status.ExpirationPolicy)
		assert.Equal(t, -1, status.DaysUntilExpiration)
		assert.False(t, status.IsExpired)
	})
}

func TestEmergencyTokenService_Revoke(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Generate token
	resp, err := svc.Generate(GenerateRequest{ExpirationDays: 90})
	require.NoError(t, err)

	// Revoke token
	err = svc.Revoke()
	assert.NoError(t, err)

	// Verify token no longer validates
	_, err = svc.Validate(resp.Token)
	assert.Error(t, err)

	// Verify no token configured
	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.False(t, status.Configured)
}

func TestEmergencyTokenService_Revoke_NoToken(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Attempt to revoke when no token exists
	err := svc.Revoke()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no token to revoke")
}

func TestEmergencyTokenService_UpdateExpiration(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Generate token with 90 days
	resp, err := svc.Generate(GenerateRequest{ExpirationDays: 90})
	require.NoError(t, err)

	// Update to 30 days
	newExpiresAt, err := svc.UpdateExpiration(30)
	require.NoError(t, err)
	assert.NotNil(t, newExpiresAt)

	// Verify updated expiration
	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, "30_days", status.ExpirationPolicy)
	assert.Greater(t, status.DaysUntilExpiration, 25)
	assert.Less(t, status.DaysUntilExpiration, 31)

	// Token should still validate
	_, err = svc.Validate(resp.Token)
	assert.NoError(t, err)
}

func TestEmergencyTokenService_UpdateExpiration_ToNever(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Generate token with 30 days
	resp, err := svc.Generate(GenerateRequest{ExpirationDays: 30})
	require.NoError(t, err)

	// Update to never expire
	newExpiresAt, err := svc.UpdateExpiration(0)
	require.NoError(t, err)
	assert.Nil(t, newExpiresAt)

	// Verify never expires
	status, err := svc.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, "never", status.ExpirationPolicy)
	assert.Equal(t, -1, status.DaysUntilExpiration)
	assert.False(t, status.IsExpired)

	// Token should still validate
	_, err = svc.Validate(resp.Token)
	assert.NoError(t, err)
}

func TestEmergencyTokenService_UpdateExpiration_NoToken(t *testing.T) {
	db := setupEmergencyTokenTestDB(t)
	svc := NewEmergencyTokenService(db)

	// Attempt to update when no token exists
	_, err := svc.UpdateExpiration(60)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no token found")
}

func TestEmergencyToken_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *time.Time
		isExpired bool
	}{
		{
			name:      "never expires",
			expiresAt: nil,
			isExpired: false,
		},
		{
			name:      "expires in future",
			expiresAt: func() *time.Time { t := time.Now().Add(24 * time.Hour); return &t }(),
			isExpired: false,
		},
		{
			name:      "expires in past",
			expiresAt: func() *time.Time { t := time.Now().Add(-24 * time.Hour); return &t }(),
			isExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &models.EmergencyToken{
				ExpiresAt: tt.expiresAt,
			}
			assert.Equal(t, tt.isExpired, token.IsExpired())
		})
	}
}

func TestEmergencyToken_DaysUntilExpiration(t *testing.T) {
	tests := []struct {
		name         string
		expiresAt    *time.Time
		expectedDays int
	}{
		{
			name:         "never expires",
			expiresAt:    nil,
			expectedDays: -1,
		},
		{
			name:         "expires in 10 days",
			expiresAt:    func() *time.Time { t := time.Now().Add(10 * 24 * time.Hour); return &t }(),
			expectedDays: 10,
		},
		{
			name:         "expired 5 days ago",
			expiresAt:    func() *time.Time { t := time.Now().Add(-5 * 24 * time.Hour); return &t }(),
			expectedDays: -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &models.EmergencyToken{
				ExpiresAt: tt.expiresAt,
			}
			days := token.DaysUntilExpiration()
			// Allow +/- 1 day for test timing variations
			assert.InDelta(t, float64(tt.expectedDays), float64(days), 1.0)
		})
	}
}
