package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupDomainCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.Domain{})
	return db
}

func TestDomainHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupDomainCoverageDB(t)
	h := NewDomainHandler(db, nil)

	// Drop table to cause error
	db.Migrator().DropTable(&models.Domain{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to fetch domains")
}

func TestDomainHandler_Create_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupDomainCoverageDB(t)
	h := NewDomainHandler(db, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/domains", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 400, w.Code)
}

func TestDomainHandler_Create_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupDomainCoverageDB(t)
	h := NewDomainHandler(db, nil)

	// Drop table to cause error
	db.Migrator().DropTable(&models.Domain{})

	body, _ := json.Marshal(map[string]string{"name": "example.com"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/domains", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to create domain")
}

func TestDomainHandler_Delete_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupDomainCoverageDB(t)
	h := NewDomainHandler(db, nil)

	// Drop table to cause error
	db.Migrator().DropTable(&models.Domain{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}

	h.Delete(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to delete domain")
}

// Remote Server Handler Tests

func setupRemoteServerCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.RemoteServer{})
	return db
}

func TestRemoteServerHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	// Drop table to cause error
	db.Migrator().DropTable(&models.RemoteServer{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/remote-servers", nil)

	h.List(c)

	assert.Equal(t, 500, w.Code)
}

func TestRemoteServerHandler_List_EnabledOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	// Create some servers
	db.Create(&models.RemoteServer{Name: "Server1", Host: "localhost", Port: 22, Enabled: true})
	db.Create(&models.RemoteServer{Name: "Server2", Host: "localhost", Port: 22, Enabled: false})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/remote-servers?enabled=true", nil)

	h.List(c)

	assert.Equal(t, 200, w.Code)
}

func TestRemoteServerHandler_Update_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: "nonexistent"}}

	h.Update(c)

	assert.Equal(t, 404, w.Code)
}

func TestRemoteServerHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	// Create a server first
	server := &models.RemoteServer{Name: "Test", Host: "localhost", Port: 22}
	svc.Create(server)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: server.UUID}}
	c.Request = httptest.NewRequest("PUT", "/remote-servers/"+server.UUID, bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
}

func TestRemoteServerHandler_TestConnection_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: "nonexistent"}}

	h.TestConnection(c)

	assert.Equal(t, 404, w.Code)
}

func TestRemoteServerHandler_TestConnectionCustom_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/remote-servers/test", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.TestConnectionCustom(c)

	assert.Equal(t, 400, w.Code)
}

func TestRemoteServerHandler_TestConnectionCustom_Unreachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"host": "192.0.2.1", // TEST-NET - should be unreachable
		"port": 65535,
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/remote-servers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.TestConnectionCustom(c)

	// Should return 200 with reachable: false
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "reachable")
}

// Uptime Handler Tests

func setupUptimeCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{})
	return db
}

func TestUptimeHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUptimeCoverageDB(t)
	svc := services.NewUptimeService(db, nil)
	h := NewUptimeHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.UptimeMonitor{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to list monitors")
}

func TestUptimeHandler_GetHistory_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUptimeCoverageDB(t)
	svc := services.NewUptimeService(db, nil)
	h := NewUptimeHandler(svc)

	// Drop history table
	db.Migrator().DropTable(&models.UptimeHeartbeat{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("GET", "/uptime/test-id/history", nil)

	h.GetHistory(c)

	assert.Equal(t, 500, w.Code)
}

func TestUptimeHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUptimeCoverageDB(t)
	svc := services.NewUptimeService(db, nil)
	h := NewUptimeHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/uptime/test-id", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
}

func TestUptimeHandler_Sync_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUptimeCoverageDB(t)
	svc := services.NewUptimeService(db, nil)
	h := NewUptimeHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.UptimeMonitor{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.Sync(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to sync monitors")
}

func TestUptimeHandler_Delete_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUptimeCoverageDB(t)
	svc := services.NewUptimeService(db, nil)
	h := NewUptimeHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.UptimeMonitor{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}

	h.Delete(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to delete monitor")
}

func TestUptimeHandler_CheckMonitor_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUptimeCoverageDB(t)
	svc := services.NewUptimeService(db, nil)
	h := NewUptimeHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	h.CheckMonitor(c)

	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "Monitor not found")
}
