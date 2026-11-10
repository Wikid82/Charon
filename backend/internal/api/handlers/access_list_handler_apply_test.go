package handlers

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// accessListApplyRouter wires the access list routes to a handler that applies the proxy config through mgr.
func accessListApplyRouter(t *testing.T, mgr CaddyConfigManager) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AccessList{}, &models.ProxyHost{}))

	h := NewAccessListHandlerWithDeps(db, mgr)
	r := gin.New()
	r.POST("/access-lists", h.Create)
	r.PUT("/access-lists/:id", h.Update)
	r.DELETE("/access-lists/:id", h.Delete)
	return r, db
}

// createAccessListForApply stores a whitelist with one rule and returns its ID.
func createAccessListForApply(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	acl := &models.AccessList{
		UUID:    "apply-acl",
		Name:    "Office",
		Type:    "whitelist",
		IPRules: `[{"cidr":"10.0.0.0/8"}]`,
		Enabled: true,
	}
	require.NoError(t, db.Create(acl).Error)
	return acl.ID
}

// attachToProxyHost makes a proxy host use the access list, which is what puts the list into the proxy config.
func attachToProxyHost(t *testing.T, db *gorm.DB, aclID uint) {
	t.Helper()
	host := &models.ProxyHost{
		UUID:         "apply-host",
		Name:         "app",
		DomainNames:  "app.example.com",
		ForwardHost:  "app",
		ForwardPort:  8080,
		Enabled:      true,
		AccessListID: &aclID,
	}
	require.NoError(t, db.Create(host).Error)
}

func putAccessList(r *gin.Engine, id uint, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/access-lists/"+strconv.FormatUint(uint64(id), 10), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

const updatedWhitelistBody = `{"name":"Office","type":"whitelist","ip_rules":"[{\"cidr\":\"192.168.0.0/16\"}]","enabled":true}`

func TestAccessListHandler_Update_AttachedListAppliesProxyConfig(t *testing.T) {
	mgr := &countingCaddyManager{}
	r, db := accessListApplyRouter(t, mgr)
	id := createAccessListForApply(t, db)
	attachToProxyHost(t, db, id)

	w := putAccessList(r, id, updatedWhitelistBody)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, mgr.calls, "an edit to a list attached to a host reaches the proxy")
}

func TestAccessListHandler_Update_UnattachedListSkipsApply(t *testing.T) {
	mgr := &countingCaddyManager{}
	r, db := accessListApplyRouter(t, mgr)
	id := createAccessListForApply(t, db)

	w := putAccessList(r, id, updatedWhitelistBody)

	require.Equal(t, http.StatusOK, w.Code)
	require.Zero(t, mgr.calls, "a list no host uses does not change the proxy config")
}

func TestAccessListHandler_Update_ApplyFailureIsReported(t *testing.T) {
	mgr := &countingCaddyManager{applyErr: errors.New("caddy refused the config")}
	r, db := accessListApplyRouter(t, mgr)
	id := createAccessListForApply(t, db)
	attachToProxyHost(t, db, id)

	w := putAccessList(r, id, updatedWhitelistBody)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "saved, but applying the configuration failed")
	require.Equal(t, 1, mgr.calls)

	var stored models.AccessList
	require.NoError(t, db.First(&stored, id).Error)
	require.Contains(t, stored.IPRules, "192.168.0.0/16", "the edit is saved even when the apply fails")
}

func TestAccessListHandler_Update_RefusedEditSkipsApply(t *testing.T) {
	mgr := &countingCaddyManager{}
	r, db := accessListApplyRouter(t, mgr)
	id := createAccessListForApply(t, db)
	attachToProxyHost(t, db, id)

	// An enabled whitelist with no rules would deny every request, so the service refuses it before any apply.
	w := putAccessList(r, id, `{"name":"Office","type":"whitelist","ip_rules":"[]","enabled":true}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Zero(t, mgr.calls)
}

func TestAccessListHandler_Delete_UnattachedListSkipsApply(t *testing.T) {
	mgr := &countingCaddyManager{}
	r, db := accessListApplyRouter(t, mgr)
	id := createAccessListForApply(t, db)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/access-lists/"+strconv.FormatUint(uint64(id), 10), http.NoBody))

	require.Equal(t, http.StatusOK, w.Code)
	require.Zero(t, mgr.calls, "only an unattached list can be deleted, so the proxy config does not change")
}

func TestAccessListHandler_Delete_AttachedListIsRefused(t *testing.T) {
	mgr := &countingCaddyManager{}
	r, db := accessListApplyRouter(t, mgr)
	id := createAccessListForApply(t, db)
	attachToProxyHost(t, db, id)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/access-lists/"+strconv.FormatUint(uint64(id), 10), http.NoBody))

	require.Equal(t, http.StatusConflict, w.Code)
	require.Zero(t, mgr.calls)
}

func TestAccessListHandler_Update_WithoutManagerSkipsApply(t *testing.T) {
	r, db := accessListApplyRouter(t, nil)
	id := createAccessListForApply(t, db)
	attachToProxyHost(t, db, id)

	w := putAccessList(r, id, updatedWhitelistBody)

	require.Equal(t, http.StatusOK, w.Code)
}
