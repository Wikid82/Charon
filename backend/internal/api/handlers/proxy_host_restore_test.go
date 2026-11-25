package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// errRefused is the apply error a refusing proxy reports in these tests.
var errRefused = errors.New("caddy refused the module")

// refusingApplier stands in for the proxy manager. ApplyConfig runs onApply first (a competing write, say) and then
// returns err.
type refusingApplier struct {
	err     error
	onApply func()
	left    map[string]struct{}
	mu      sync.Mutex
	seen    []context.Context
}

func (a *refusingApplier) ApplyConfig(ctx context.Context) error {
	a.mu.Lock()
	a.seen = append(a.seen, ctx)
	a.mu.Unlock()
	if a.onApply != nil {
		a.onApply()
	}
	return a.err
}

func (a *refusingApplier) LeftOutHostUUIDs() map[string]struct{} { return a.left }

// hostRestoreFixture is a proxy host handler wired to an applier, over a database with the tables host writes need.
type hostRestoreFixture struct {
	router  *gin.Engine
	db      *gorm.DB
	handler *ProxyHostHandler
}

// openHostRestoreDB opens an in-memory database with the proxy host tables migrated.
func openHostRestoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
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
	return db
}

// newHostRestoreFixture builds the handler over db, with role as the caller and applier as the proxy manager.
func newHostRestoreFixture(t *testing.T, db *gorm.DB, role string, applier *refusingApplier) hostRestoreFixture {
	t.Helper()
	h := NewProxyHostHandler(db, nil, nil, nil)
	h.caddyManager = applier
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, role)
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"))
	redirect := NewRedirectionHostHandler(db, nil)
	redirect.caddyManager = applier
	redirect.RegisterRoutes(r.Group("/api/v1"))
	return hostRestoreFixture{router: r, db: db, handler: h}
}

func (f hostRestoreFixture) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

// seedHost stores a host with two locations and returns it with its ids.
func seedHost(t *testing.T, db *gorm.DB, name string) models.ProxyHost {
	t.Helper()
	host := models.ProxyHost{
		UUID:        uuid.NewString(),
		Name:        name,
		DomainNames: name + ".restore.example.com",
		ForwardHost: "10.0.0.5",
		ForwardPort: 8080,
		Enabled:     true,
		Locations: []models.Location{
			{UUID: uuid.NewString(), Path: "/a", ForwardHost: "10.0.0.5", ForwardPort: 9001},
			{UUID: uuid.NewString(), Path: "/b", ForwardHost: "10.0.0.5", ForwardPort: 9002},
		},
	}
	require.NoError(t, services.NewProxyHostService(db).CreateAs(&host, true))
	return host
}

func storedHost(t *testing.T, db *gorm.DB, id uint) models.ProxyHost {
	t.Helper()
	var host models.ProxyHost
	require.NoError(t, db.First(&host, id).Error)
	return host
}

func locationCount(t *testing.T, db *gorm.DB, hostID uint) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.Location{}).Where("proxy_host_id = ?", hostID).Count(&n).Error)
	return n
}

func TestProxyHostCreateRefusedLeavesNoRow(t *testing.T) {
	db := openHostRestoreDB(t)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodPost, "/api/v1/proxy-hosts", map[string]any{
		"name":         "refused-create",
		"domain_names": "refused-create.example.com",
		"forward_host": "10.0.0.6",
		"forward_port": 8081,
		"locations": []map[string]any{
			{"path": "/x", "forward_host": "10.0.0.6", "forward_port": 9100},
		},
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := decodeBody(t, w)
	assert.Equal(t, "proxy_apply_failed", body["error_code"])
	assert.Contains(t, body["error"], "previous settings kept")
	assert.Contains(t, body["error"], errRefused.Error(), "an administrator sees the apply detail")

	var hosts, locations int64
	require.NoError(t, db.Model(&models.ProxyHost{}).Count(&hosts).Error)
	require.NoError(t, db.Model(&models.Location{}).Count(&locations).Error)
	assert.Zero(t, hosts, "the refused host is removed again")
	assert.Zero(t, locations, "its locations are removed with it")
}

func TestProxyHostCreateRefusedForStandardUserHidesTheDetail(t *testing.T) {
	db := openHostRestoreDB(t)
	f := newHostRestoreFixture(t, db, string(models.RoleUser), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodPost, "/api/v1/proxy-hosts", map[string]any{
		"name":         "refused-user",
		"domain_names": "refused-user.example.com",
		"forward_host": "10.0.0.6",
		"forward_port": 8081,
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := decodeBody(t, w)
	assert.Equal(t, "Failed to apply configuration (previous settings kept)", body["error"])
	assert.NotContains(t, w.Body.String(), errRefused.Error())
	assert.Equal(t, "proxy_apply_failed", body["error_code"])
}

func TestProxyHostUpdateRefusedRestoresThePreviousRow(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "refused-update")
	before := storedHost(t, db, host.ID)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, map[string]any{"forward_port": 9999, "forward_host": "10.9.9.9"})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Equal(t, "proxy_apply_failed", decodeBody(t, w)["error_code"])
	after := storedHost(t, db, host.ID)
	assert.Equal(t, before.ForwardPort, after.ForwardPort)
	assert.Equal(t, before.ForwardHost, after.ForwardHost)
	assert.Equal(t, int64(2), locationCount(t, db, host.ID))
}

func TestProxyHostUpdateRefusedKeepsWriteMadeSinceTheApply(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "two-writers")

	// The competing write lands between the refused write and the restore.
	applier := &refusingApplier{err: errRefused, onApply: func() {
		other := services.NewProxyHostService(db)
		current, err := other.GetByID(host.ID)
		require.NoError(t, err)
		current.ForwardHost = "10.7.7.7"
		require.NoError(t, other.UpdateAs(current, true))
	}}
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), applier)

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, map[string]any{"forward_port": 9999})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	after := storedHost(t, db, host.ID)
	assert.Equal(t, "10.7.7.7", after.ForwardHost, "the newer write is kept")
	assert.Equal(t, 9999, after.ForwardPort, "the refused change is not rolled back over the newer write")
}

func TestProxyHostDeleteRefusedPutsTheHostBack(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "refused-delete")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodDelete, "/api/v1/proxy-hosts/"+host.UUID, nil)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	restored := storedHost(t, db, host.ID)
	assert.Equal(t, host.UUID, restored.UUID, "the host comes back with its id and uuid")
	assert.Equal(t, host.DomainNames, restored.DomainNames)
	assert.Equal(t, int64(2), locationCount(t, db, host.ID), "its locations come back")
}

func TestProxyHostDeleteRefusedSkipsReinsertWhenDomainIsTaken(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "taken-domain")

	// While the refused delete is being undone, another host claims the same domain.
	applier := &refusingApplier{err: errRefused, onApply: func() {
		claim := models.ProxyHost{
			UUID:        uuid.NewString(),
			DomainNames: host.DomainNames,
			ForwardHost: "10.1.1.1",
			ForwardPort: 1,
		}
		require.NoError(t, db.Create(&claim).Error)
	}}
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), applier)

	w := f.do(t, http.MethodDelete, "/api/v1/proxy-hosts/"+host.UUID, nil)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	var count int64
	require.NoError(t, db.Model(&models.ProxyHost{}).Where("uuid = ?", host.UUID).Count(&count).Error)
	assert.Zero(t, count, "the deleted host is not re-inserted over the new claim")
}

func TestProxyHostBulkACLRefusedRestoresTheColumn(t *testing.T) {
	db := openHostRestoreDB(t)
	seedAccessListsForBulk(t, db, 7)
	first := seedHost(t, db, "bulk-acl-1")
	second := seedHost(t, db, "bulk-acl-2")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", map[string]any{
		"host_uuids":     []string{first.UUID, second.UUID},
		"access_list_id": 7,
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := decodeBody(t, w)
	assert.Equal(t, float64(0), body["updated"])
	assert.Equal(t, "proxy_apply_failed", body["error_code"])
	assert.Nil(t, storedHost(t, db, first.ID).AccessListID)
	assert.Nil(t, storedHost(t, db, second.ID).AccessListID)
}

func TestProxyHostBulkACLRefusedKeepsTheNewerValueOfOneHost(t *testing.T) {
	db := openHostRestoreDB(t)
	seedAccessListsForBulk(t, db, 7)
	first := seedHost(t, db, "bulk-acl-keep-1")
	second := seedHost(t, db, "bulk-acl-keep-2")

	// A competing edit reaches the first host only, after the bulk write and before the restore.
	applier := &refusingApplier{err: errRefused, onApply: func() {
		other := services.NewProxyHostService(db)
		current, err := other.GetByID(first.ID)
		require.NoError(t, err)
		newer := uint(9)
		current.AccessListID = &newer
		require.NoError(t, other.UpdateAs(current, true))
	}}
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), applier)

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", map[string]any{
		"host_uuids":     []string{first.UUID, second.UUID},
		"access_list_id": 7,
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	kept := storedHost(t, db, first.ID).AccessListID
	require.NotNil(t, kept)
	assert.Equal(t, uint(9), *kept, "the newer value is kept")
	assert.Nil(t, storedHost(t, db, second.ID).AccessListID, "the other host is restored")
}

func TestProxyHostBulkGroupRefusedRestoresTheColumn(t *testing.T) {
	db := openHostRestoreDB(t)
	group := models.ProxyGroup{UUID: uuid.NewString(), Name: "restore-group"}
	require.NoError(t, db.Create(&group).Error)
	host := seedHost(t, db, "bulk-group")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-group", map[string]any{
		"host_uuids":     []string{host.UUID},
		"proxy_group_id": group.UUID,
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Equal(t, float64(0), decodeBody(t, w)["updated"])
	assert.Nil(t, storedHost(t, db, host.ID).ProxyGroupID)
}

func TestProxyHostBulkSecurityHeadersRefusedRestoresTheColumn(t *testing.T) {
	db := openHostRestoreDB(t)
	profile := models.SecurityHeaderProfile{UUID: uuid.NewString(), Name: "restore-profile"}
	require.NoError(t, db.Create(&profile).Error)
	host := seedHost(t, db, "bulk-headers")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", map[string]any{
		"host_uuids":                 []string{host.UUID},
		"security_header_profile_id": profile.ID,
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Equal(t, float64(0), decodeBody(t, w)["updated"])
	assert.Nil(t, storedHost(t, db, host.ID).SecurityHeaderProfileID)
}

func TestProxyHostListAndGetWithoutAManagerDoNotPanic(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "no-manager")
	h := NewProxyHostHandler(db, nil, nil, nil)
	require.Nil(t, h.caddyManager, "a nil manager is a nil interface")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, string(models.RoleAdmin))
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"))

	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/proxy-hosts", http.NoBody))
	assert.Equal(t, http.StatusOK, list.Code)

	get := httptest.NewRecorder()
	r.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/proxy-hosts/"+host.UUID, http.NoBody))
	assert.Equal(t, http.StatusOK, get.Code)
}

// seedAccessListsForBulk creates the access lists a bulk ACL test assigns, with the ids it sends. A bulk assignment
// refuses a list that does not exist.
func seedAccessListsForBulk(t *testing.T, db *gorm.DB, ids ...uint) {
	t.Helper()
	for _, id := range ids {
		require.NoError(t, db.Create(&models.AccessList{
			ID: id, UUID: uuid.NewString(), Name: fmt.Sprintf("bulk-acl-%d", id), Type: "ip", Enabled: true,
		}).Error)
	}
}
