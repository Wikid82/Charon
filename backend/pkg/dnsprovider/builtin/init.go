package builtin

import (
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// init automatically registers all built-in DNS provider plugins when the package is imported.
func init() {
	providers := []dnsprovider.ProviderPlugin{
		&CloudflareProvider{},
		&Route53Provider{},
		&DigitalOceanProvider{},
		&GoogleCloudDNSProvider{},
		&AzureProvider{},
		&NamecheapProvider{},
		&GoDaddyProvider{},
		&HetznerProvider{},
		&VultrProvider{},
		&DNSimpleProvider{},
	}

	for _, provider := range providers {
		if err := provider.Init(); err != nil {
			logger.Log().WithError(err).Warnf("Failed to initialize built-in provider: %s", provider.Type())
			continue
		}

		if err := dnsprovider.Global().Register(provider); err != nil {
			logger.Log().WithError(err).Warnf("Failed to register built-in provider: %s", provider.Type())
			continue
		}

		logger.Log().Debugf("Registered built-in DNS provider: %s", provider.Type())
	}
}
