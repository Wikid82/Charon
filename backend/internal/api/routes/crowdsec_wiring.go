package routes

import (
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/caddy"
)

// newCrowdsecHandler builds the CrowdSec handler wired to the proxy. Start re-applies the proxy configuration
// through caddyManager, so the router always passes the manager it serves; without it Start cannot apply.
func newCrowdsecHandler(db *gorm.DB, exec handlers.CrowdsecExecutor, binPath, dataDir string, caddyManager *caddy.Manager) *handlers.CrowdsecHandler {
	return handlers.NewCrowdsecHandler(db, exec, binPath, dataDir, caddyManager)
}
