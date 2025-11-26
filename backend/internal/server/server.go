package server

import (
	"github.com/gin-gonic/gin"
)

// NewRouter creates a new Gin router with frontend static file serving.
func NewRouter(frontendDir string) *gin.Engine {
	router := gin.Default()
	// Silence "trusted all proxies" warning by not trusting any by default.
	// If running behind a proxy, this should be configured to trust that proxy's IP.
	_ = router.SetTrustedProxies(nil)

	// Serve frontend static files
	if frontendDir != "" {
		router.Static("/assets", frontendDir+"/assets")
		router.StaticFile("/", frontendDir+"/index.html")
		router.StaticFile("/banner.png", frontendDir+"/banner.png")
		router.StaticFile("/logo.png", frontendDir+"/logo.png")
		router.StaticFile("/favicon.png", frontendDir+"/favicon.png")
		router.NoRoute(func(c *gin.Context) {
			c.File(frontendDir + "/index.html")
		})
	}

	return router
}
