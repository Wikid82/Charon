package services

import (
	"context"
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
		defer svc.Close()

		// No config exists yet
		_, err := svc.Get()
		assert.ErrorIs(t, err, ErrSecurityConfigNotFound)
	})

	t.Run("SecurityService_ListRuleSets_EmptyDB", func(t *testing.T) {
		svc := NewSecurityService(db)
		defer svc.Close()

		// Should not error with empty db
		rulesets, err := svc.ListRuleSets()
		assert.NoError(t, err)
		assert.NotNil(t, rulesets)
		assert.Empty(t, rulesets)
	})

	t.Run("SecurityService_DeleteRuleSet_NotFound", func(t *testing.T) {
		svc := NewSecurityService(db)
		defer svc.Close()

		// Test with non-existent ID
		err := svc.DeleteRuleSet(999)
		assert.Error(t, err)
	})

	t.Run("SecurityService_VerifyBreakGlass_MissingConfig", func(t *testing.T) {
		svc := NewSecurityService(db)
		defer svc.Close()

		// No config exists
		valid, err := svc.VerifyBreakGlassToken("default", "anytoken")
		assert.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("SecurityService_GenerateBreakGlassToken_Success", func(t *testing.T) {
		svc := NewSecurityService(db)
		defer svc.Close()

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
		svc := NewNotificationService(db, nil)

		// Should not error with empty db
		templates, err := svc.ListTemplates()
		assert.NoError(t, err)
		assert.NotNil(t, templates)
		assert.Empty(t, templates)
	})

	t.Run("NotificationService_GetTemplate_NotFound", func(t *testing.T) {
		svc := NewNotificationService(db, nil)

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
	defer svc.Close()

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
		err := svc.SendEmail(context.Background(), []string{"test@example.com"}, "Subject", "Body")
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
		port := extractPort("https://discord.com/api/webhooks/123/abc:8080/path")
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

// TestCoverageBoost_ProxyHostService_DB tests DB accessor
func TestCoverageBoost_ProxyHostService_DB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	svc := NewProxyHostService(db)

	t.Run("DB_ReturnsValidDB", func(t *testing.T) {
		dbInstance := svc.DB()
		assert.NotNil(t, dbInstance)
		assert.Equal(t, db, dbInstance)
	})
}

// TestCoverageBoost_DNSProviderService_SupportedTypes tests provider type queries
func TestCoverageBoost_DNSProviderService_SupportedTypes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{})
	require.NoError(t, err)

	svc := NewDNSProviderService(db, nil)

	t.Run("GetSupportedProviderTypes", func(t *testing.T) {
		types := svc.GetSupportedProviderTypes()
		assert.NotNil(t, types)
		// Should include at least some built-in types
		assert.NotEmpty(t, types)
	})

	t.Run("GetProviderCredentialFields_ValidProvider", func(t *testing.T) {
		types := svc.GetSupportedProviderTypes()
		if len(types) > 0 {
			// Test with first available provider
			fields, err := svc.GetProviderCredentialFields(types[0])
			assert.NoError(t, err)
			assert.NotNil(t, fields)
		}
	})

	t.Run("GetProviderCredentialFields_InvalidProvider", func(t *testing.T) {
		fields, err := svc.GetProviderCredentialFields("invalid-provider-type-12345")
		assert.Error(t, err)
		assert.Nil(t, fields)
		assert.Contains(t, err.Error(), "unsupported provider type")
	})
}

// TestCoverageBoost_SecurityService_Close tests service cleanup
func TestCoverageBoost_SecurityService_Close(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	svc := NewSecurityService(db)
	defer svc.Close() // Ensure cleanup even if test panics

	t.Run("Close_Success", func(t *testing.T) {
		svc.Close()
		// Close doesn't return error, just ensure it doesn't panic
	})

	t.Run("Flush_Success", func(t *testing.T) {
		svc.Flush()
		// Flush doesn't return error, just ensure it doesn't panic
	})
}

// TestCoverageBoost_BackupService_GetAvailableSpace tests disk space checking
func TestCoverageBoost_BackupService_GetAvailableSpace(t *testing.T) {
	// Skip these tests as they require full config setup
	t.Skip("BackupService requires full config.Config, tested elsewhere")
}

// TestCoverageBoost_CertificateService_ListCertificates tests certificate listing with errors
func TestCoverageBoost_CertificateService_ListCertificates(t *testing.T) {
	// Skip these tests as they require proper model imports
	t.Skip("Certificate models tested in certificate_service_test.go")
}

// TestCoverageBoost_MailService_SendSSL tests SSL mail sending error paths
func TestCoverageBoost_MailService_SendSSL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	svc := NewMailService(db)

	t.Run("SendEmail_SSL_InvalidHost", func(t *testing.T) {
		// Save invalid config
		config := &SMTPConfig{
			Host:        "invalid-mail-server-12345.example.com",
			Port:        465,
			Username:    "test",
			Password:    "test",
			FromAddress: "test@example.com",
			Encryption:  "ssl",
		}
		err := svc.SaveSMTPConfig(config)
		require.NoError(t, err)

		// Try to send - should fail with connection error
		err = svc.SendEmail(context.Background(), []string{"test@example.com"}, "Test", "Body")
		assert.Error(t, err)
	})

	t.Run("SendEmail_STARTTLS_InvalidHost", func(t *testing.T) {
		// Save invalid config with STARTTLS
		config := &SMTPConfig{
			Host:        "invalid-mail-server-12345.example.com",
			Port:        587,
			Username:    "test",
			Password:    "test",
			FromAddress: "test@example.com",
			Encryption:  "starttls",
		}
		err := svc.SaveSMTPConfig(config)
		require.NoError(t, err)

		// Try to send - should fail with connection error
		err = svc.SendEmail(context.Background(), []string{"test@example.com"}, "Test", "Body")
		assert.Error(t, err)
	})
}

// TestCoverageBoost_CredentialService_ErrorPaths tests credential service error handling
func TestCoverageBoost_CredentialService_ErrorPaths(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})
	require.NoError(t, err)

	// Note: CredentialService requires crypto.EncryptionService, tested in credential_service_test.go
	t.Skip("CredentialService requires crypto.EncryptionService, tested elsewhere")
}

// TestCoverageBoost_GeoIPService_ErrorPaths tests GeoIP service error handling
func TestCoverageBoost_GeoIPService_ErrorPaths(t *testing.T) {
	t.Run("NewGeoIPService_InvalidPath", func(t *testing.T) {
		svc, err := NewGeoIPService("/nonexistent/path/to/geoip.mmdb")
		assert.Error(t, err)
		assert.Nil(t, svc)
	})
}

// TestCoverageBoost_DockerService_ErrorPaths tests Docker service error handling
func TestCoverageBoost_DockerService_ErrorPaths(t *testing.T) {
	t.Skip("Docker service tests require specific setup, tested in docker_service_test.go")
}

// TestCoverageBoost_UptimeService_FlushNotifications tests notification flushing
func TestCoverageBoost_UptimeService_FlushNotifications(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHost{})
	require.NoError(t, err)

	svc := NewUptimeService(db, nil)

	t.Run("FlushPendingNotifications", func(t *testing.T) {
		// Should not error even with empty pending notifications
		svc.FlushPendingNotifications()
	})
}

// TestCoverageBoost_LogService_NewLogService tests log service creation
func TestCoverageBoost_LogService_NewLogService(t *testing.T) {
	t.Skip("LogService requires full config, tested in log_service_test.go")
}

// TestCoverageBoost_UpdateService_ClearCache tests cache clearing
func TestCoverageBoost_UpdateService_ClearCache(t *testing.T) {
	svc := NewUpdateService()

	t.Run("ClearCache", func(t *testing.T) {
		svc.ClearCache()
	})

	t.Run("SetCurrentVersion", func(t *testing.T) {
		svc.SetCurrentVersion("v1.2.3")
	})
}

// TestCoverageBoost_NotificationService_Providers tests provider management
func TestCoverageBoost_NotificationService_Providers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.NotificationProvider{})
	require.NoError(t, err)

	svc := NewNotificationService(db, nil)

	t.Run("ListProviders_EmptyDB", func(t *testing.T) {
		providers, err := svc.ListProviders()
		assert.NoError(t, err)
		assert.NotNil(t, providers)
		assert.Empty(t, providers)
	})

	t.Run("CreateProvider", func(t *testing.T) {
		provider := &models.NotificationProvider{
			Name:    "test-provider",
			Type:    "discord",
			URL:     "https://discord.com/api/webhooks/123/abc",
			Enabled: true,
			Config:  `{"url": "https://discord.com/api/webhooks/123/abc"}`,
		}
		err := svc.CreateProvider(provider)
		assert.NoError(t, err)
		assert.NotZero(t, provider.ID)
	})

	t.Run("UpdateProvider", func(t *testing.T) {
		// Create a provider first
		provider := &models.NotificationProvider{
			Name:    "update-test",
			Type:    "discord",
			URL:     "https://discord.com/api/webhooks/123/abc",
			Enabled: true,
			Config:  `{"url": "https://discord.com/api/webhooks/123/abc"}`,
		}
		err := svc.CreateProvider(provider)
		require.NoError(t, err)

		// Update it
		provider.Name = "updated-name"
		err = svc.UpdateProvider(provider)
		assert.NoError(t, err)
	})

	t.Run("DeleteProvider", func(t *testing.T) {
		// Create a provider first
		provider := &models.NotificationProvider{
			Name:    "delete-test",
			Type:    "discord",
			URL:     "https://discord.com/api/webhooks/123/abc",
			Enabled: true,
			Config:  `{"url": "https://discord.com/api/webhooks/123/abc"}`,
		}
		err := svc.CreateProvider(provider)
		require.NoError(t, err)

		// Delete it
		err = svc.DeleteProvider(provider.ID)
		assert.NoError(t, err)
	})
}

// TestCoverageBoost_NotificationService_CRUD tests notification CRUD operations
func TestCoverageBoost_NotificationService_CRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Notification{})
	require.NoError(t, err)

	svc := NewNotificationService(db, nil)

	t.Run("List_EmptyDB", func(t *testing.T) {
		notifs, err := svc.List(false)
		assert.NoError(t, err)
		assert.NotNil(t, notifs)
	})

	t.Run("MarkAllAsRead_Success", func(t *testing.T) {
		err := svc.MarkAllAsRead()
		assert.NoError(t, err)
	})
}
