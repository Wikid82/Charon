package routes_test

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
)

// bodyLimitedRoutes lists every route that reads a request body and must carry the body limit: the proxy and
// redirection host writes, the two settings URL checks and the import upload, commit and cancel routes.
var bodyLimitedRoutes = []string{
	"POST /api/v1/proxy-hosts",
	"PUT /api/v1/proxy-hosts/:uuid",
	"POST /api/v1/proxy-hosts/test",
	"PUT /api/v1/proxy-hosts/bulk-update-acl",
	"PUT /api/v1/proxy-hosts/bulk-update-group",
	"PUT /api/v1/proxy-hosts/bulk-update-security-headers",
	"POST /api/v1/redirection-hosts",
	"PUT /api/v1/redirection-hosts/:uuid",
	"POST /api/v1/settings/validate-url",
	"POST /api/v1/settings/test-url",
	"POST /api/v1/import/upload",
	"POST /api/v1/import/upload-multi",
	"POST /api/v1/import/detect-imports",
	"POST /api/v1/import/commit",
	"DELETE /api/v1/import/cancel",
	"POST /api/v1/import/json/upload",
	"POST /api/v1/import/json/commit",
	"POST /api/v1/import/json/cancel",
	"POST /api/v1/import/npm/upload",
	"POST /api/v1/import/npm/commit",
	"POST /api/v1/import/npm/cancel",
}

// bodyLimitHandlerName is part of the name of the handler that caps a request body (handlers.limitRequestBody).
const bodyLimitHandlerName = "handlers.limitRequestBody"

// routeHandlerChains maps each registered route ("METHOD /full/path") to the names of its handler chain. It walks the
// engine's routing trees, which gin keeps unexported, so the walk reads them by reflection.
func routeHandlerChains(engine *gin.Engine) map[string][]string {
	chains := map[string][]string{}
	trees := reflect.ValueOf(engine).Elem().FieldByName("trees")
	for i := 0; i < trees.Len(); i++ {
		tree := trees.Index(i)
		collectHandlerChains(tree.FieldByName("root"), tree.FieldByName("method").String(), chains)
	}
	return chains
}

// collectHandlerChains records the handler chain of every node below node that ends a route.
func collectHandlerChains(node reflect.Value, method string, chains map[string][]string) {
	if node.IsNil() {
		return
	}
	n := node.Elem()
	handlers := n.FieldByName("handlers")
	if fullPath := n.FieldByName("fullPath").String(); fullPath != "" && handlers.Len() > 0 {
		names := make([]string, 0, handlers.Len())
		for j := 0; j < handlers.Len(); j++ {
			names = append(names, runtime.FuncForPC(handlers.Index(j).Pointer()).Name())
		}
		chains[method+" "+fullPath] = names
	}
	children := n.FieldByName("children")
	for k := 0; k < children.Len(); k++ {
		collectHandlerChains(children.Index(k), method, chains)
	}
}

// TestRegister_BodyLimitIsAttachedToEveryBodyRoute walks the real router and checks that each body-reading route has
// the body limit in its handler chain, so a route added later without it is caught here.
func TestRegister_BodyLimitIsAttachedToEveryBodyRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(routes.IsolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)

	router := gin.New()
	require.NoError(t, routes.Register(context.Background(), router, db, config.Config{JWTSecret: "test-secret"}))
	routes.RegisterImportHandler(router, db, config.Config{JWTSecret: "test-secret"}, "", t.TempDir(), "", nil)
	chains := routeHandlerChains(router)

	for _, route := range bodyLimitedRoutes {
		names, registered := chains[route]
		if !assert.True(t, registered, "route %s is registered", route) {
			continue
		}
		assert.Contains(t, strings.Join(names, " "), bodyLimitHandlerName, "route %s caps its request body", route)
	}

	// Negative control: a route that reads no body must not carry the limit, so the walk does not match everything.
	assert.NotContains(t, strings.Join(chains["GET /api/v1/proxy-hosts"], " "), bodyLimitHandlerName)
}
