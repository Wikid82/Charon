// Package custom provides custom DNS provider plugins for non-built-in integrations.
package custom

import (
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// init automatically registers all custom DNS provider plugins when the package is imported.
func init() {
	providers := []dnsprovider.ProviderPlugin{
		NewManualProvider(),
		NewRFC2136Provider(),
		NewWebhookProvider(),
		NewScriptProvider(),
	}

	for _, provider := range providers {
		if err := provider.Init(); err != nil {
			logger.Log().WithError(err).Warnf("Failed to initialize custom provider: %s", provider.Type())
			continue
		}

		if err := dnsprovider.Global().Register(provider); err != nil {
			logger.Log().WithError(err).Warnf("Failed to register custom provider: %s", provider.Type())
			continue
		}

		logger.Log().Debugf("Registered custom DNS provider: %s", provider.Type())
	}
}
