package caddy

import (
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

// GenerateConfigOption configures optional inputs to GenerateConfig that
// don't belong in its long positional parameter list. Introduced by the
// Redirection Hosts feature specifically so adding RedirectionHost input
// does not require a breaking positional-parameter insertion — see
// docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md §4.2.
//
// This replaces GenerateConfig's previous trailing variadic
// (encSvc ...*crypto.EncryptionService). Every call site that omitted that
// variadic keeps compiling unchanged, since a variadic parameter's type
// change doesn't affect callers that pass zero values for it; only the
// handful of call sites that supplied an explicit *crypto.EncryptionService
// value need to switch to WithEncryptionService(...).
type GenerateConfigOption func(*generateConfigOptions)

// generateConfigOptions holds the resolved values of every GenerateConfigOption
// passed to GenerateConfig.
type generateConfigOptions struct {
	encSvc        *crypto.EncryptionService
	redirectHosts []models.RedirectionHost
	selfHop       *selfHopConfig
	remoteHosts   map[string]struct{}
}

// resolveGenerateConfigOptions applies opts in order and returns the
// resolved options struct GenerateConfig reads from internally.
func resolveGenerateConfigOptions(opts []GenerateConfigOption) *generateConfigOptions {
	resolved := &generateConfigOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(resolved)
		}
	}
	return resolved
}

// WithEncryptionService supplies the optional encryption service GenerateConfig
// previously received via its bare *crypto.EncryptionService variadic parameter.
// Used to decrypt custom certificates' private keys during config generation.
func WithEncryptionService(svc *crypto.EncryptionService) GenerateConfigOption {
	return func(o *generateConfigOptions) { o.encSvc = svc }
}

// WithRedirectionHosts supplies enabled RedirectionHost rows for
// BuildRedirectRoutes and combined TLS/Ghost-Host domain handling.
func WithRedirectionHosts(hosts []models.RedirectionHost) GenerateConfigOption {
	return func(o *generateConfigOptions) { o.redirectHosts = hosts }
}

// WithSelfHop enables hop headers on proxy routes whose upstream is Charon's own
// API listening on port. Without it no route carries them.
func WithSelfHop(secret *selfhop.Secret, port string) GenerateConfigOption {
	return func(o *generateConfigOptions) {
		o.selfHop = &selfHopConfig{secret: secret, port: port, local: selfhop.NewLocalAddrs()}
	}
}

// WithRemoteHosts lists proxy host UUIDs whose upstream lives on another
// machine (such as an agent-resolved target). Those routes never receive hop
// headers, whatever address the upstream resolved to.
func WithRemoteHosts(uuids map[string]struct{}) GenerateConfigOption {
	return func(o *generateConfigOptions) { o.remoteHosts = uuids }
}

func (o *generateConfigOptions) isRemoteHost(uuid string) bool {
	_, ok := o.remoteHosts[uuid]
	return ok
}
