package caddy

import (
	charonconfig "github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
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
	// adminListen is the admin.listen value of the generated config. It is set to the default socket when no option supplies it.
	adminListen string
	selfHop     *selfHopConfig
	enforcement func(EnforcementReport)
	// bouncerKeyPath is the saved CrowdSec bouncer key file the proxy reads.
	bouncerKeyPath string
	// geoIPPath is the configured GeoIP database file, and geoIPAvailable says whether the proxy may use it now.
	geoIPPath      string
	geoIPAvailable bool
}

// resolveGenerateConfigOptions applies opts in order and returns the
// resolved options struct GenerateConfig reads from internally.
func resolveGenerateConfigOptions(opts []GenerateConfigOption) *generateConfigOptions {
	resolved := &generateConfigOptions{bouncerKeyPath: bouncerkey.DefaultFile}
	for _, opt := range opts {
		if opt != nil {
			opt(resolved)
		}
	}
	if resolved.adminListen == "" {
		resolved.adminListen = charonconfig.CaddyAdminListen(charonconfig.DefaultCaddyAdminAPI())
	}
	return resolved
}

// WithAdminListen sets the admin.listen value of the generated config. Pass the
// value that matches the admin API endpoint the client dials (see
// config.CaddyAdminListen). Without it the default admin socket is used.
func WithAdminListen(listen string) GenerateConfigOption {
	return func(o *generateConfigOptions) { o.adminListen = listen }
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

// WithBouncerKeyPath sets the saved CrowdSec bouncer key file the proxy reads. Manager passes
// bouncerkey.PathFor(the CrowdSec data directory), the file the handlers and boot use. An empty path keeps
// the image default, bouncerkey.DefaultFile.
func WithBouncerKeyPath(path string) GenerateConfigOption {
	return func(o *generateConfigOptions) {
		if path != "" {
			o.bouncerKeyPath = path
		}
	}
}

// WithEnforcementSink receives the enforcement report of one GenerateConfig call. The sink is called on every return
// path, so a caller never keeps a report from an earlier generation.
func WithEnforcementSink(sink func(EnforcementReport)) GenerateConfigOption {
	return func(o *generateConfigOptions) { o.enforcement = sink }
}

// WithGeoIPDatabase supplies the GeoIP database the proxy's country lists read and whether it is usable now. The
// caller decides usability with geoip.DatabaseAvailable. Without this option no database is available, so every
// country list is refused.
func WithGeoIPDatabase(path string, available bool) GenerateConfigOption {
	return func(o *generateConfigOptions) {
		o.geoIPPath = path
		o.geoIPAvailable = available
	}
}
