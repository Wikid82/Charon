package services

import (
	"net"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestCoverageBoost_ErrorPaths tests various error handling paths to increase coverage
func TestCoverageBoost_ErrorPaths(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	// Migrate all tables
	err = db.AutoMigrate(
		&models.ProxyHost{},
		&models.RemoteServer{},
		&models.SecurityConfig{},
		&models.SecurityRuleSet{},
		&models.NotificationTemplate{},
		&models.Setting{},
	)
	require.NoError(t, err)

	t.Run("ProxyHostService_GetByUUID_Error", func(t *testing.T) {
		svc := NewProxyHostService(db)

		// Test with non-existent UUID
		_, err := svc.GetByUUID("non-existent-uuid")
		assert.Error(t, err)
	})

	t.Run("ProxyHostService_List_WithValidDB", func(t *testing.T) {
		svc := NewProxyHostService(db)

		// Should not error even with empty db
		hosts, err := svc.List()
		assert.NoError(t, err)
		assert.NotNil(t, hosts)
	})

	t.Run("RemoteServerService_GetByUUID_Error", func(t *testing.T) {
		svc := NewRemoteServerService(db)

		// Test with non-existent UUID
		_, err := svc.GetByUUID("non-existent-uuid")
		assert.Error(t, err)
	})

	t.Run("RemoteServerService_List_WithValidDB", func(t *testing.T) {
		svc := NewRemoteServerService(db)

		// Should not error with empty db
		servers, err := svc.List(false)
		assert.NoError(t, err)
		assert.NotNil(t, servers)
	})

	t.Run("SecurityService_Get_NotFound", func(t *testing.T) {
		svc := NewSecurityService(db)

		// No config exists yet
		_, err := svc.Get()
		assert.ErrorIs(t, err, ErrSecurityConfigNotFound)
	})

	t.Run("SecurityService_ListRuleSets_EmptyDB", func(t *testing.T) {
		svc := NewSecurityService(db)

		// Should not error with empty db
		rulesets, err := svc.ListRuleSets()
		assert.NoError(t, err)
		assert.NotNil(t, rulesets)
		assert.Empty(t, rulesets)
	})

	t.Run("SecurityService_DeleteRuleSet_NotFound", func(t *testing.T) {
		svc := NewSecurityService(db)

		// Test with non-existent ID
		err := svc.DeleteRuleSet(999)
		assert.Error(t, err)
	})

	t.Run("SecurityService_VerifyBreakGlass_MissingConfig", func(t *testing.T) {
		svc := NewSecurityService(db)

		// No config exists
		valid, err := svc.VerifyBreakGlassToken("default", "anytoken")
		assert.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("SecurityService_GenerateBreakGlassToken_Success", func(t *testing.T) {
		svc := NewSecurityService(db)

		// Generate token
		token, err := svc.GenerateBreakGlassToken("test-config")
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		// Verify it was created
		var cfg models.SecurityConfig
		err = db.Where("name = ?", "test-config").First(&cfg).Error
		assert.NoError(t, err)
		assert.NotEmpty(t, cfg.BreakGlassHash)
	})

	t.Run("NotificationService_ListTemplates_EmptyDB", func(t *testing.T) {
		svc := NewNotificationService(db)

		// Should not error with empty db
		templates, err := svc.ListTemplates()
		assert.NoError(t, err)
		assert.NotNil(t, templates)
		assert.Empty(t, templates)
	})

	t.Run("NotificationService_GetTemplate_NotFound", func(t *testing.T) {
		svc := NewNotificationService(db)

		// Test with non-existent ID
		_, err := svc.GetTemplate("nonexistent")
		assert.Error(t, err)
	})
}

// TestCoverageBoost_SecurityService_AdditionalPaths tests more security service paths
func TestCoverageBoost_SecurityService_AdditionalPaths(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityRuleSet{})
	require.NoError(t, err)

	svc := NewSecurityService(db)

	t.Run("Upsert_Create", func(t *testing.T) {
		// Create initial config
		cfg := &models.SecurityConfig{
			Name:         "default",
			CrowdSecMode: "local",
		}
		err := svc.Upsert(cfg)
		require.NoError(t, err)
	})

	t.Run("UpsertRuleSet_Create", func(t *testing.T) {
		ruleset := &models.SecurityRuleSet{
			Name:      "test-ruleset-new",
			SourceURL: "https://example.com",
		}
		err := svc.UpsertRuleSet(ruleset)
		assert.NoError(t, err)

		// Verify created
		var found models.SecurityRuleSet
		err = db.Where("name = ?", "test-ruleset-new").First(&found).Error
		assert.NoError(t, err)
	})
}

// TestCoverageBoost_MinInt tests the minInt helper
func TestCoverageBoost_MinInt(t *testing.T) {
	t.Run("minInt_FirstSmaller", func(t *testing.T) {
		result := minInt(5, 10)
		assert.Equal(t, 5, result)
	})

	t.Run("minInt_SecondSmaller", func(t *testing.T) {
		result := minInt(10, 5)
		assert.Equal(t, 5, result)
	})

	t.Run("minInt_Equal", func(t *testing.T) {
		result := minInt(5, 5)
		assert.Equal(t, 5, result)
	})
}

// TestCoverageBoost_MailService_ErrorPaths tests mail service error handling
func TestCoverageBoost_MailService_ErrorPaths(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	svc := NewMailService(db)

	t.Run("GetSMTPConfig_EmptyDB", func(t *testing.T) {
		// Empty DB should return config with defaults
		config, err := svc.GetSMTPConfig()
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})

	t.Run("IsConfigured_NoConfig", func(t *testing.T) {
		// With empty DB, should return false
		configured := svc.IsConfigured()
		assert.False(t, configured)
	})

	t.Run("TestConnection_NoConfig", func(t *testing.T) {
		// With empty config, should error
		err := svc.TestConnection()
		assert.Error(t, err)
	})

	t.Run("SendEmail_NoConfig", func(t *testing.T) {
		// With empty config, should error
		err := svc.SendEmail("test@example.com", "Subject", "Body")
		assert.Error(t, err)
	})
}

// TestCoverageBoost_AccessListService_Paths tests access list error paths
func TestCoverageBoost_AccessListService_Paths(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.AccessList{})
	require.NoError(t, err)

	svc := NewAccessListService(db)

	t.Run("GetByID_NotFound", func(t *testing.T) {
		_, err := svc.GetByID(999)
		assert.ErrorIs(t, err, ErrAccessListNotFound)
	})

	t.Run("GetByUUID_NotFound", func(t *testing.T) {
		_, err := svc.GetByUUID("nonexistent-uuid")
		assert.ErrorIs(t, err, ErrAccessListNotFound)
	})

	t.Run("List_EmptyDB", func(t *testing.T) {
		// Should not error with empty db
		lists, err := svc.List()
		assert.NoError(t, err)
		assert.NotNil(t, lists)
		assert.Empty(t, lists)
	})
}

// TestCoverageBoost_HelperFunctions tests utility helper functions
func TestCoverageBoost_HelperFunctions(t *testing.T) {
	t.Run("extractPort_HTTP", func(t *testing.T) {
		port := extractPort("http://example.com:8080/path")
		assert.Equal(t, "8080", port)
	})

	t.Run("extractPort_HTTPS", func(t *testing.T) {
		port := extractPort("https://example.com:443")
		assert.Equal(t, "443", port)
	})

	t.Run("extractPort_Invalid", func(t *testing.T) {
		port := extractPort("not-a-url")
		assert.Equal(t, "", port)
	})

	t.Run("hasHeader_Found", func(t *testing.T) {
		headers := map[string][]string{
			"X-Test-Header": {"value1", "value2"},
			"Content-Type":  {"application/json"},
		}
		assert.True(t, hasHeader(headers, "X-Test-Header"))
		assert.True(t, hasHeader(headers, "Content-Type"))
	})

	t.Run("hasHeader_NotFound", func(t *testing.T) {
		headers := map[string][]string{
			"X-Test-Header": {"value1"},
		}
		assert.False(t, hasHeader(headers, "X-Missing-Header"))
	})

	t.Run("hasHeader_EmptyMap", func(t *testing.T) {
		headers := map[string][]string{}
		assert.False(t, hasHeader(headers, "Any-Header"))
	})

	t.Run("isPrivateIP_PrivateRanges", func(t *testing.T) {
		assert.True(t, isPrivateIP(net.ParseIP("192.168.1.1")))
		assert.True(t, isPrivateIP(net.ParseIP("10.0.0.1")))
		assert.True(t, isPrivateIP(net.ParseIP("172.16.0.1")))
		assert.True(t, isPrivateIP(net.ParseIP("127.0.0.1")))
	})

	t.Run("isPrivateIP_PublicIP", func(t *testing.T) {
		assert.False(t, isPrivateIP(net.ParseIP("8.8.8.8")))
		assert.False(t, isPrivateIP(net.ParseIP("1.1.1.1")))
	})
}
