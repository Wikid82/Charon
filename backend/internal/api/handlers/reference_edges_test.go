package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// deleteBeforeHostWrite runs del just before every write to proxy_hosts. It stands in for a delete that lands between
// the handler's reference check and its write. Every write runs in its own transaction, so a delete made for a failed
// write is rolled back with it; running del before each write keeps the referenced row gone for the next host too.
func deleteBeforeHostWrite(t *testing.T, db *gorm.DB, del func(tx *gorm.DB) error) {
	t.Helper()
	const callbackName = "test:delete-before-host-write"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "proxy_hosts" {
			return
		}
		if err := del(tx.Session(&gorm.Session{NewDB: true})); err != nil {
			_ = tx.AddError(err)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })
}

// putHost sends an update for the host with uuid and returns the recorded response.
func putHost(t *testing.T, router http.Handler, path, role string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if role != "" {
		req.Header.Set("X-Test-Role", role)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

// A host update whose access list or header profile is deleted between the check and the write is refused with 409
// reference_missing, and the host keeps its old value.
func TestProxyHostUpdate_ReferenceDeletedBeforeWrite_Returns409(t *testing.T) {
	t.Run("access list", func(t *testing.T) {
		router, db := setupTestRouterWithReferenceTables(t)
		host := createTestProxyHost(t, db, "acl-moved-host")
		list := models.AccessList{UUID: uuid.NewString(), Name: "office"}
		require.NoError(t, db.Create(&list).Error)
		deleteBeforeHostWrite(t, db, func(tx *gorm.DB) error {
			return tx.Delete(&models.AccessList{}, list.ID).Error
		})

		resp := putHost(t, router, "/api/v1/proxy-hosts/"+host.UUID, "", map[string]any{
			"name": "acl-moved-host", "domain_names": "acl-moved-host.test.com",
			"forward_scheme": "http", "forward_host": "localhost", "forward_port": 8080,
			"access_list_id": list.ID,
		})

		require.Equal(t, http.StatusConflict, resp.Code, resp.Body.String())
		assert.Contains(t, resp.Body.String(), "reference_missing")
		var stored models.ProxyHost
		require.NoError(t, db.First(&stored, host.ID).Error)
		assert.Nil(t, stored.AccessListID, "the host keeps its old access list")
	})

	t.Run("security header profile", func(t *testing.T) {
		router, db := setupTestRouterWithReferenceTables(t)
		host := createTestProxyHost(t, db, "profile-moved-host")
		profile := models.SecurityHeaderProfile{UUID: uuid.NewString(), Name: "strict"}
		require.NoError(t, db.Create(&profile).Error)
		deleteBeforeHostWrite(t, db, func(tx *gorm.DB) error {
			return tx.Delete(&models.SecurityHeaderProfile{}, profile.ID).Error
		})

		resp := putHost(t, router, "/api/v1/proxy-hosts/"+host.UUID, "", map[string]any{
			"name": "profile-moved-host", "domain_names": "profile-moved-host.test.com",
			"forward_scheme": "http", "forward_host": "localhost", "forward_port": 8080,
			"security_header_profile_id": profile.ID,
		})

		require.Equal(t, http.StatusConflict, resp.Code, resp.Body.String())
		assert.Contains(t, resp.Body.String(), "reference_missing")
		var stored models.ProxyHost
		require.NoError(t, db.First(&stored, host.ID).Error)
		assert.Nil(t, stored.SecurityHeaderProfileID)
	})
}

// A bulk access list assignment whose list is deleted during the loop reports each host as failed with the fixed text,
// and the response stays 200.
func TestProxyHostBulkACL_ListDeletedMidLoop_ReportsFixedFailureText(t *testing.T) {
	router, db := setupTestRouterWithReferenceTables(t)
	first := createTestProxyHost(t, db, "bulk-moved-one")
	second := createTestProxyHost(t, db, "bulk-moved-two")
	list := models.AccessList{UUID: uuid.NewString(), Name: "bulk-moved"}
	require.NoError(t, db.Create(&list).Error)
	deleteBeforeHostWrite(t, db, func(tx *gorm.DB) error {
		return tx.Delete(&models.AccessList{}, list.ID).Error
	})

	body := fmt.Sprintf(`{"host_uuids":["%s","%s"],"access_list_id":%d}`, first.UUID, second.UUID, list.ID)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var result struct {
		Updated int                 `json:"updated"`
		Errors  []map[string]string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Zero(t, result.Updated)
	require.Len(t, result.Errors, 2)
	for _, failure := range result.Errors {
		assert.Equal(t, "a selected item was deleted", failure["error"])
	}
}

// A standard user can save a host whose DNS provider was removed by the repair. The repair clears the provider and turns
// the DNS challenge off, so the admin-only fields still match what is stored.
func TestPermissionProxyHost_StandardUserSavesHostRepairedByDNSRule(t *testing.T) {
	p := setupHostPermsRouter(t)
	stored := p.seedProxyHost(t, "repaired")
	require.NoError(t, p.db.Delete(&models.DNSProvider{}, p.dns.ID).Error)

	results, err := database.RepairReferences(p.db)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	var repaired models.ProxyHost
	require.NoError(t, p.db.Where("uuid = ?", stored.UUID).First(&repaired).Error)
	require.Nil(t, repaired.DNSProviderID, "precondition: the repair cleared the provider")
	require.False(t, repaired.UseDNSChallenge, "precondition: the repair turned the DNS challenge off")

	rec := p.do(t, http.MethodPut, "/api/v1/proxy-hosts/"+stored.UUID, "user", map[string]any{
		"name":              "renamed after repair",
		"dns_provider_id":   nil,
		"use_dns_challenge": false,
		"forward_port":      9090,
	})

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var after models.ProxyHost
	require.NoError(t, p.db.Where("uuid = ?", stored.UUID).First(&after).Error)
	assert.Equal(t, "renamed after repair", after.Name)
}

// With reference checks on, deleting a host removes its locations in the database. When the apply is refused, the host
// and its locations come back.
func TestProxyHostDeleteRefusedWithReferenceChecksOnRestoresLocations(t *testing.T) {
	db := testutil.OpenMemoryDBWithForeignKeys(t)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.AccessList{},
		&models.ProxyGroup{},
		&models.SecurityHeaderProfile{},
		&models.RedirectionHost{},
		&models.Notification{},
		&models.NotificationProvider{},
	))
	host := models.ProxyHost{
		UUID: uuid.NewString(), Name: "cascade-host", DomainNames: "cascade-host.example.com",
		ForwardHost: "10.0.0.5", ForwardPort: 8080, Enabled: true,
		Locations: []models.Location{
			{UUID: uuid.NewString(), Path: "/a", ForwardHost: "10.0.0.5", ForwardPort: 9001},
			{UUID: uuid.NewString(), Path: "/b", ForwardHost: "10.0.0.5", ForwardPort: 9002},
		},
	}
	require.NoError(t, db.Create(&host).Error)

	var locationsAtApply int64 = -1
	applier := &refusingApplier{err: errRefused, onApply: func() {
		var n int64
		if err := db.Model(&models.Location{}).Where("proxy_host_id = ?", host.ID).Count(&n).Error; err == nil {
			locationsAtApply = n
		}
	}}
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), applier)

	w := f.do(t, http.MethodDelete, "/api/v1/proxy-hosts/"+host.UUID, nil)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Zero(t, locationsAtApply, "the database removed the locations before the apply")
	restored := storedHost(t, db, host.ID)
	assert.Equal(t, host.UUID, restored.UUID)
	assert.Equal(t, int64(2), locationCount(t, db, host.ID), "the locations come back with the host")
}
