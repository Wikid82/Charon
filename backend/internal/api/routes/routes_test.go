package routes

import (
"testing"

"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
"github.com/gin-gonic/gin"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
"gorm.io/driver/sqlite"
"gorm.io/gorm"
)

func TestRegister(t *testing.T) {
gin.SetMode(gin.TestMode)
router := gin.New()

// Use in-memory DB
db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
require.NoError(t, err)

cfg := config.Config{
JWTSecret: "test-secret",
}

err = Register(router, db, cfg)
assert.NoError(t, err)

// Verify some routes are registered
routes := router.Routes()
assert.NotEmpty(t, routes)

foundHealth := false
for _, r := range routes {
if r.Path == "/api/v1/health" {
foundHealth = true
break
}
}
assert.True(t, foundHealth, "Health route should be registered")
}
