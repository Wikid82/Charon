package services

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.AccessList{}, &models.ProxyHost{})
	assert.NoError(t, err)

	return db
}

func TestAccessListService_Create(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	t.Run("create whitelist with valid IP rules", func(t *testing.T) {
		rules := []models.AccessListRule{
			{CIDR: "192.168.1.0/24", Description: "Home network"},
			{CIDR: "10.0.0.1", Description: "Single IP"},
		}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:        "Test Whitelist",
			Description: "Test description",
			Type:        "whitelist",
			IPRules:     string(rulesJSON),
			Enabled:     true,
		}

		err := service.Create(acl)
		assert.NoError(t, err)
		assert.NotEmpty(t, acl.UUID)
		assert.NotZero(t, acl.ID)
	})

	t.Run("create geo whitelist with valid country codes", func(t *testing.T) {
		acl := &models.AccessList{
			Name:         "US Only",
			Description:  "Allow only US",
			Type:         "geo_whitelist",
			CountryCodes: "US",
			Enabled:      true,
		}

		err := service.Create(acl)
		assert.NoError(t, err)
		assert.NotEmpty(t, acl.UUID)
	})

	t.Run("create local network only ACL", func(t *testing.T) {
		acl := &models.AccessList{
			Name:             "Local Network",
			Description:      "RFC1918 only",
			Type:             "whitelist",
			LocalNetworkOnly: true,
			Enabled:          true,
		}

		err := service.Create(acl)
		assert.NoError(t, err)
		assert.NotEmpty(t, acl.UUID)
	})

	t.Run("fail with empty name", func(t *testing.T) {
		acl := &models.AccessList{
			Name:    "",
			Type:    "whitelist",
			Enabled: true,
		}

		err := service.Create(acl)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("fail with invalid type", func(t *testing.T) {
		acl := &models.AccessList{
			Name:    "Test",
			Type:    "invalid_type",
			Enabled: true,
		}

		err := service.Create(acl)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidAccessListType, err)
	})

	t.Run("fail with invalid IP address", func(t *testing.T) {
		rules := []models.AccessListRule{
			{CIDR: "invalid-ip", Description: "Bad IP"},
		}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Test",
			Type:    "whitelist",
			IPRules: string(rulesJSON),
			Enabled: true,
		}

		err := service.Create(acl)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidIPAddress)
	})

	t.Run("fail geo-blocking without country codes", func(t *testing.T) {
		acl := &models.AccessList{
			Name:         "Geo Fail",
			Type:         "geo_whitelist",
			CountryCodes: "",
			Enabled:      true,
		}

		err := service.Create(acl)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "country codes are required")
	})

	t.Run("fail with invalid country code", func(t *testing.T) {
		acl := &models.AccessList{
			Name:         "Invalid Country",
			Type:         "geo_whitelist",
			CountryCodes: "XX",
			Enabled:      true,
		}

		err := service.Create(acl)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidCountryCode)
	})
}

func TestAccessListService_GetByID(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	// Create test ACL
	acl := &models.AccessList{
		Name:    "Test ACL",
		Type:    "whitelist",
		Enabled: true,
	}
	err := service.Create(acl)
	assert.NoError(t, err)

	t.Run("get existing ACL", func(t *testing.T) {
		found, err := service.GetByID(acl.ID)
		assert.NoError(t, err)
		assert.Equal(t, acl.ID, found.ID)
		assert.Equal(t, acl.Name, found.Name)
	})

	t.Run("get non-existent ACL", func(t *testing.T) {
		_, err := service.GetByID(99999)
		assert.Error(t, err)
		assert.Equal(t, ErrAccessListNotFound, err)
	})
}

func TestAccessListService_GetByUUID(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	// Create test ACL
	acl := &models.AccessList{
		Name:    "Test ACL",
		Type:    "whitelist",
		Enabled: true,
	}
	err := service.Create(acl)
	assert.NoError(t, err)

	t.Run("get existing ACL by UUID", func(t *testing.T) {
		found, err := service.GetByUUID(acl.UUID)
		assert.NoError(t, err)
		assert.Equal(t, acl.UUID, found.UUID)
		assert.Equal(t, acl.Name, found.Name)
	})

	t.Run("get non-existent ACL by UUID", func(t *testing.T) {
		_, err := service.GetByUUID("non-existent-uuid")
		assert.Error(t, err)
		assert.Equal(t, ErrAccessListNotFound, err)
	})
}

func TestAccessListService_GetByID_DBError(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	sqlDB, err := db.DB()
	assert.NoError(t, err)
	assert.NoError(t, sqlDB.Close())

	_, err = service.GetByID(1)
	assert.Error(t, err)
}

func TestAccessListService_GetByUUID_DBError(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	sqlDB, err := db.DB()
	assert.NoError(t, err)
	assert.NoError(t, sqlDB.Close())

	_, err = service.GetByUUID("any")
	assert.Error(t, err)
}

func TestAccessListService_List(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	// Create multiple ACLs
	acl1 := &models.AccessList{Name: "ACL 1", Type: "whitelist", Enabled: true}
	acl2 := &models.AccessList{Name: "ACL 2", Type: "blacklist", Enabled: true}

	err := service.Create(acl1)
	assert.NoError(t, err)
	err = service.Create(acl2)
	assert.NoError(t, err)

	t.Run("list all ACLs", func(t *testing.T) {
		acls, err := service.List()
		assert.NoError(t, err)
		assert.Len(t, acls, 2)
	})

	t.Run("list uses deterministic id desc tie-breaker", func(t *testing.T) {
		fixed := time.Date(2026, time.February, 13, 10, 0, 0, 0, time.UTC)
		assert.NoError(t, db.Model(&models.AccessList{}).Where("id IN ?", []uint{acl1.ID, acl2.ID}).Update("updated_at", fixed).Error)

		acls, err := service.List()
		assert.NoError(t, err)
		assert.Len(t, acls, 2)
		assert.Equal(t, acl2.ID, acls[0].ID)
		assert.Equal(t, acl1.ID, acls[1].ID)
	})
}

func TestAccessListService_Update(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	// Create test ACL
	acl := &models.AccessList{
		Name:    "Original Name",
		Type:    "whitelist",
		Enabled: true,
	}
	err := service.Create(acl)
	assert.NoError(t, err)

	t.Run("update successfully", func(t *testing.T) {
		updates := &models.AccessList{
			Name:        "Updated Name",
			Description: "Updated description",
			Type:        "blacklist",
			Enabled:     false,
		}

		err := service.Update(acl.ID, updates)
		assert.NoError(t, err)

		// Verify updates
		updated, _ := service.GetByID(acl.ID)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, "Updated description", updated.Description)
		assert.Equal(t, "blacklist", updated.Type)
		assert.False(t, updated.Enabled)
	})

	t.Run("fail update on non-existent ACL", func(t *testing.T) {
		updates := &models.AccessList{Name: "Test", Type: "whitelist", Enabled: true}
		err := service.Update(99999, updates)
		assert.Error(t, err)
		assert.Equal(t, ErrAccessListNotFound, err)
	})

	t.Run("fail update with invalid data", func(t *testing.T) {
		updates := &models.AccessList{Name: "", Type: "whitelist", Enabled: true}
		err := service.Update(acl.ID, updates)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
}

func TestAccessListService_Delete(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	t.Run("delete successfully", func(t *testing.T) {
		acl := &models.AccessList{Name: "To Delete", Type: "whitelist", Enabled: true}
		err := service.Create(acl)
		assert.NoError(t, err)

		err = service.Delete(acl.ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = service.GetByID(acl.ID)
		assert.Error(t, err)
		assert.Equal(t, ErrAccessListNotFound, err)
	})

	t.Run("fail delete non-existent ACL", func(t *testing.T) {
		err := service.Delete(99999)
		assert.Error(t, err)
		assert.Equal(t, ErrAccessListNotFound, err)
	})

	t.Run("fail delete ACL in use", func(t *testing.T) {
		// Create ACL
		acl := &models.AccessList{Name: "In Use", Type: "whitelist", Enabled: true}
		err := service.Create(acl)
		assert.NoError(t, err)

		// Create proxy host using the ACL
		host := &models.ProxyHost{
			UUID:          "test-uuid",
			DomainNames:   "example.com",
			ForwardScheme: "http",
			ForwardHost:   "localhost",
			ForwardPort:   8080,
			AccessListID:  &acl.ID,
		}
		err = db.Create(host).Error
		assert.NoError(t, err)

		// Try to delete ACL
		err = service.Delete(acl.ID)
		assert.Error(t, err)
		assert.Equal(t, ErrAccessListInUse, err)
	})
}

func TestAccessListService_TestIP(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	t.Run("whitelist allows matching IP", func(t *testing.T) {
		rules := []models.AccessListRule{{CIDR: "192.168.1.0/24"}}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Whitelist",
			Type:    "whitelist",
			IPRules: string(rulesJSON),
			Enabled: true,
		}
		err := service.Create(acl)
		assert.NoError(t, err)

		allowed, reason, err := service.TestIP(acl.ID, "192.168.1.100")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "Allowed by whitelist")
	})

	t.Run("whitelist blocks non-matching IP", func(t *testing.T) {
		rules := []models.AccessListRule{{CIDR: "192.168.1.0/24"}}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Whitelist",
			Type:    "whitelist",
			IPRules: string(rulesJSON),
			Enabled: true,
		}
		err := service.Create(acl)
		assert.NoError(t, err)

		allowed, reason, err := service.TestIP(acl.ID, "10.0.0.1")
		assert.NoError(t, err)
		assert.False(t, allowed)
		assert.Contains(t, reason, "Not in whitelist")
	})

	t.Run("blacklist blocks matching IP", func(t *testing.T) {
		rules := []models.AccessListRule{{CIDR: "10.0.0.0/8"}}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Blacklist",
			Type:    "blacklist",
			IPRules: string(rulesJSON),
			Enabled: true,
		}
		err := service.Create(acl)
		assert.NoError(t, err)

		allowed, reason, err := service.TestIP(acl.ID, "10.0.0.1")
		assert.NoError(t, err)
		assert.False(t, allowed)
		assert.Contains(t, reason, "Blocked by blacklist")
	})

	t.Run("blacklist allows non-matching IP", func(t *testing.T) {
		rules := []models.AccessListRule{{CIDR: "10.0.0.0/8"}}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Blacklist",
			Type:    "blacklist",
			IPRules: string(rulesJSON),
			Enabled: true,
		}
		err := service.Create(acl)
		assert.NoError(t, err)

		allowed, reason, err := service.TestIP(acl.ID, "192.168.1.1")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "Not in blacklist")
	})

	t.Run("local network only allows RFC1918", func(t *testing.T) {
		acl := &models.AccessList{
			Name:             "Local Only",
			Type:             "whitelist",
			LocalNetworkOnly: true,
			Enabled:          true,
		}
		err := service.Create(acl)
		assert.NoError(t, err)

		// Test private IP
		allowed, _, err := service.TestIP(acl.ID, "192.168.1.1")
		assert.NoError(t, err)
		assert.True(t, allowed)

		// Test public IP
		allowed, reason, err := service.TestIP(acl.ID, "8.8.8.8")
		assert.NoError(t, err)
		assert.False(t, allowed)
		assert.Contains(t, reason, "Not a private network IP")
	})

	t.Run("disabled ACL allows all", func(t *testing.T) {
		rules := []models.AccessListRule{{CIDR: "192.168.1.0/24"}}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Disabled",
			Type:    "whitelist",
			IPRules: string(rulesJSON),
			Enabled: false, // Disabled
		}
		err := service.Create(acl)
		assert.NoError(t, err)

		allowed, reason, err := service.TestIP(acl.ID, "10.0.0.1")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "disabled")
	})

	t.Run("fail with invalid IP", func(t *testing.T) {
		acl := &models.AccessList{Name: "Test", Type: "whitelist", Enabled: true}
		err := service.Create(acl)
		assert.NoError(t, err)

		_, _, err = service.TestIP(acl.ID, "invalid-ip")
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidIPAddress, err)
	})
}

func TestAccessListService_GetTemplates(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	templates := service.GetTemplates()
	assert.NotEmpty(t, templates)
	assert.GreaterOrEqual(t, len(templates), 3)

	// Check structure of first template
	first := templates[0]
	assert.Contains(t, first, "name")
	assert.Contains(t, first, "description")
	assert.Contains(t, first, "type")
}

func TestAccessListService_Validation(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	t.Run("validate CIDR formats", func(t *testing.T) {
		validCIDRs := []string{
			"192.168.1.0/24",
			"10.0.0.1",
			"172.16.0.0/12",
			"2001:db8::/32",
			"::1",
		}

		for _, cidr := range validCIDRs {
			assert.True(t, service.isValidCIDR(cidr), "CIDR should be valid: %s", cidr)
		}

		invalidCIDRs := []string{
			"256.0.0.1",
			"192.168.1.0/33",
			"invalid",
			"",
		}

		for _, cidr := range invalidCIDRs {
			assert.False(t, service.isValidCIDR(cidr), "CIDR should be invalid: %s", cidr)
		}
	})

	t.Run("validate country codes", func(t *testing.T) {
		validCodes := []string{"US", "GB", "CA", "DE", "FR"}
		for _, code := range validCodes {
			assert.True(t, service.isValidCountryCode(code), "Country code should be valid: %s", code)
		}

		invalidCodes := []string{"XX", "USA", "1", "", "G"}
		for _, code := range invalidCodes {
			assert.False(t, service.isValidCountryCode(code), "Country code should be invalid: %s", code)
		}
	})

	t.Run("validate types", func(t *testing.T) {
		validTypes := []string{"whitelist", "blacklist", "geo_whitelist", "geo_blacklist"}
		for _, typ := range validTypes {
			assert.True(t, service.isValidType(typ), "Type should be valid: %s", typ)
		}

		invalidTypes := []string{"invalid", "allow", "deny", ""}
		for _, typ := range invalidTypes {
			assert.False(t, service.isValidType(typ), "Type should be invalid: %s", typ)
		}
	})
}

// TestIPMatchesCIDR_Helper tests the ipMatchesCIDR helper function
func TestIPMatchesCIDR_Helper(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	tests := []struct {
		name    string
		ipStr   string
		cidr    string
		matches bool
	}{
		{"IPv4 in subnet", "192.168.1.50", "192.168.1.0/24", true},
		{"IPv4 not in subnet", "192.168.2.50", "192.168.1.0/24", false},
		{"IPv4 single IP match", "10.0.0.1", "10.0.0.1", true},
		{"IPv4 single IP no match", "10.0.0.2", "10.0.0.1", false},
		{"IPv6 in subnet", "2001:db8::1", "2001:db8::/32", true},
		{"IPv6 not in subnet", "2001:db9::1", "2001:db8::/32", false},
		{"Invalid CIDR", "192.168.1.1", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ipStr)
			if ip == nil {
				t.Fatalf("Failed to parse test IP: %s", tt.ipStr)
			}
			result := service.ipMatchesCIDR(ip, tt.cidr)
			assert.Equal(t, tt.matches, result)
		})
	}
}

// TestIsPrivateIP_Helper tests the isPrivateIP helper function
func TestIsPrivateIP_Helper(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	tests := []struct {
		name      string
		ipStr     string
		isPrivate bool
	}{
		{"Private 10.x.x.x", "10.0.0.1", true},
		{"Private 172.16.x.x", "172.16.0.1", true},
		{"Private 192.168.x.x", "192.168.1.1", true},
		{"Private 127.0.0.1", "127.0.0.1", true},
		{"Private ::1", "::1", true},
		{"Private fc00::/7", "fc00::1", true},
		{"Public 8.8.8.8", "8.8.8.8", false},
		{"Public 1.1.1.1", "1.1.1.1", false},
		{"Public IPv6", "2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ipStr)
			if ip == nil {
				t.Fatalf("Failed to parse test IP: %s", tt.ipStr)
			}
			result := service.isPrivateIP(ip)
			assert.Equal(t, tt.isPrivate, result)
		})
	}
}

// TestAccessListService_ListFunction tests the List function
func TestAccessListService_ListFunction(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	// Create a few access lists
	acl1 := &models.AccessList{
		Name:    "List 1",
		Type:    "whitelist",
		Enabled: true,
	}
	acl2 := &models.AccessList{
		Name:    "List 2",
		Type:    "blacklist",
		Enabled: false,
	}

	assert.NoError(t, service.Create(acl1))
	assert.NoError(t, service.Create(acl2))

	// Test listing
	lists, err := service.List()
	assert.NoError(t, err)
	assert.Len(t, lists, 2)
}

// TestAccessListService_SetGeoIPService tests the GeoIP service setter and getter.
func TestAccessListService_SetGeoIPService(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	// Initially nil
	assert.Nil(t, service.GetGeoIPService())

	// Setting nil should work
	service.SetGeoIPService(nil)
	assert.Nil(t, service.GetGeoIPService())
}

// TestAccessListService_GeoACL_NoGeoIPService tests geo ACL behavior when GeoIP service is not available.
func TestAccessListService_GeoACL_NoGeoIPService(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)
	// Don't set GeoIP service

	t.Run("geo_whitelist without GeoIP service allows traffic", func(t *testing.T) {
		acl := &models.AccessList{
			Name:         "US Only",
			Type:         "geo_whitelist",
			CountryCodes: "US",
			Enabled:      true,
		}
		err := service.Create(acl)
		assert.NoError(t, err)

		// Should allow with graceful degradation message
		allowed, reason, err := service.TestIP(acl.ID, "8.8.8.8")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "GeoIP database not available")
	})

	t.Run("geo_blacklist without GeoIP service allows traffic", func(t *testing.T) {
		acl := &models.AccessList{
			Name:         "Block Russia",
			Type:         "geo_blacklist",
			CountryCodes: "RU",
			Enabled:      true,
		}
		err := service.Create(acl)
		assert.NoError(t, err)

		// Should allow with graceful degradation message
		allowed, reason, err := service.TestIP(acl.ID, "1.2.3.4")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "GeoIP database not available")
	})
}

// TestAccessListService_List_EmptyDatabase tests List with empty database.
func TestAccessListService_List_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	acls, err := service.List()
	assert.NoError(t, err)
	assert.Empty(t, acls)
}

// TestAccessListService_GetTemplates_Structure tests template structure and content.
func TestAccessListService_GetTemplates_Structure(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	templates := service.GetTemplates()

	t.Run("templates contain required fields", func(t *testing.T) {
		for _, template := range templates {
			assert.Contains(t, template, "name")
			assert.Contains(t, template, "description")
			assert.Contains(t, template, "type")
			assert.NotEmpty(t, template["name"])
			assert.NotEmpty(t, template["description"])
			assert.NotEmpty(t, template["type"])
		}
	})

	t.Run("templates have valid types", func(t *testing.T) {
		validTypes := map[string]bool{
			"whitelist":     true,
			"blacklist":     true,
			"geo_whitelist": true,
			"geo_blacklist": true,
		}

		for _, template := range templates {
			templateType, ok := template["type"].(string)
			assert.True(t, ok, "Template type should be a string")
			assert.True(t, validTypes[templateType], "Template type should be valid: %s", templateType)
		}
	})
}

// TestAccessListService_ParseCountryCodes tests the country code parsing helper.
func TestAccessListService_ParseCountryCodes(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	t.Run("parse single code", func(t *testing.T) {
		codes := service.parseCountryCodes("US")
		assert.Equal(t, []string{"US"}, codes)
	})

	t.Run("parse multiple codes", func(t *testing.T) {
		codes := service.parseCountryCodes("US,GB,DE")
		assert.Equal(t, []string{"US", "GB", "DE"}, codes)
	})

	t.Run("parse with spaces", func(t *testing.T) {
		codes := service.parseCountryCodes("US, GB, DE")
		assert.Equal(t, []string{"US", "GB", "DE"}, codes)
	})

	t.Run("parse with lowercase", func(t *testing.T) {
		codes := service.parseCountryCodes("us,gb,de")
		assert.Equal(t, []string{"US", "GB", "DE"}, codes)
	})

	t.Run("parse empty string", func(t *testing.T) {
		codes := service.parseCountryCodes("")
		assert.Nil(t, codes)
	})

	t.Run("parse with empty entries", func(t *testing.T) {
		codes := service.parseCountryCodes("US,,GB,,,DE")
		assert.Equal(t, []string{"US", "GB", "DE"}, codes)
	})
}

// TestAccessListService_ValidateACLRules tests ACL rule validation.
func TestAccessListService_ValidateACLRules(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	t.Run("validate valid IP rules", func(t *testing.T) {
		rules := []models.AccessListRule{
			{CIDR: "192.168.1.0/24", Description: "Valid subnet"},
			{CIDR: "10.0.0.1", Description: "Valid single IP"},
		}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Valid Rules",
			Type:    "whitelist",
			IPRules: string(rulesJSON),
			Enabled: true,
		}

		err := service.Create(acl)
		assert.NoError(t, err)
	})

	t.Run("fail validation with invalid CIDR", func(t *testing.T) {
		rules := []models.AccessListRule{
			{CIDR: "256.256.256.256", Description: "Invalid IP"},
		}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Invalid Rules",
			Type:    "whitelist",
			IPRules: string(rulesJSON),
			Enabled: true,
		}

		err := service.Create(acl)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidIPAddress)
	})

	t.Run("fail validation with mixed valid and invalid rules", func(t *testing.T) {
		rules := []models.AccessListRule{
			{CIDR: "192.168.1.0/24", Description: "Valid"},
			{CIDR: "not-an-ip", Description: "Invalid"},
		}
		rulesJSON, _ := json.Marshal(rules)

		acl := &models.AccessList{
			Name:    "Mixed Rules",
			Type:    "whitelist",
			IPRules: string(rulesJSON),
			Enabled: true,
		}

		err := service.Create(acl)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidIPAddress)
	})
}
