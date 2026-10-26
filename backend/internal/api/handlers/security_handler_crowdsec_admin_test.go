package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
)

// The CrowdSec switch routes refuse callers that are not admins before they touch the engine.
func TestCrowdSecSwitchRoutes_RefuseNonAdminCallers(t *testing.T) {
	routes := map[string]func(*SecurityHandler, *gin.Context){
		"enable":  (*SecurityHandler).EnableCrowdSec,
		"disable": (*SecurityHandler).DisableCrowdSec,
	}
	for name, call := range routes {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/security/crowdsec/"+name, http.NoBody)
			c.Set(middleware.RoleKey, "user")

			call(&SecurityHandler{}, c)

			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}
