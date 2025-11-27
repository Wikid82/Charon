package services

import (
	"encoding/json"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
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
