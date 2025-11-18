package server

import (
	"github.com/gin-gonic/gin"
)

// NewRouter creates a new Gin router with frontend static file serving.
func NewRouter(frontendDir string) *gin.Engine {
	router := gin.Default()

	// Serve frontend static files
	if frontendDir != "" {
		router.Static("/assets", frontendDir+"/assets")
		router.StaticFile("/", frontendDir+"/index.html")
		router.NoRoute(func(c *gin.Context) {
			c.File(frontendDir + "/index.html")
		})
	}

	return router
}
