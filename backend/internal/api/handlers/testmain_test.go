package handlers

import (
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/testutil/tmpguard"
	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	database.SyncIntegrityCheckForTesting()
	os.Exit(tmpguard.Run(m))
}
