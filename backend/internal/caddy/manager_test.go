package caddy

import (
"context"
"encoding/json"
"net/http"
"net/http/httptest"
"testing"

"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
"gorm.io/driver/sqlite"
"gorm.io/gorm"
)

func TestManager_ApplyConfig(t *testing.T) {
// Mock Caddy Admin API
caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.URL.Path == "/load" && r.Method == "POST" {
// Verify payload
var config Config
err := json.NewDecoder(r.Body).Decode(&config)
if err != nil {
w.WriteHeader(http.StatusBadRequest)
return
}
w.WriteHeader(http.StatusOK)
return
}
w.WriteHeader(http.StatusNotFound)
}))
defer caddyServer.Close()

// Setup DB
db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
require.NoError(t, err)
db.AutoMigrate(&models.ProxyHost{}, &models.Setting{})

// Seed DB
db.Create(&models.ProxyHost{
UUID:        "test-uuid",
DomainNames: "example.com",
ForwardHost: "localhost",
ForwardPort: 8080,
Enabled:     true,
})

client := NewClient(caddyServer.URL)
manager := NewManager(client, db, t.TempDir())

err = manager.ApplyConfig(context.Background())
assert.NoError(t, err)
}
