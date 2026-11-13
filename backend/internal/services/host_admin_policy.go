package services

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

// ErrAdminOnlyHostField is returned when a caller without administrator rights
// would change a host's DNS provider binding, its DNS challenge flag or its
// advanced configuration. The handler maps it to a 403 with error_code
// permissions_admin_only.
var ErrAdminOnlyHostField = errors.New("admin privileges required to change DNS provider, DNS challenge or advanced config")

// ErrAdminListenerTarget is returned when a forward target would reach the
// proxy's own administrative API. It applies to every caller.
var ErrAdminListenerTarget = errors.New("forward target cannot point at this proxy's own admin API")

// caddyAdminAPIPort is the port of the Caddy admin TCP listener. The default
// admin API is a unix socket with no TCP listener, so this refusal is defense in
// depth for loopback TCP setups and also refuses loopback targets on this port.
var caddyAdminAPIPort = adminAPIPortFrom(config.CaddyAdminTCPAddress)

// localHostChecker answers whether a dial host names this machine.
var localHostChecker = selfhop.NewLocalAddrs()

// hostAdminFields are the stored or incoming values of the settings that only
// administrators may change.
type hostAdminFields struct {
	dnsProviderID   *uint
	useDNSChallenge bool
	advancedConfig  string
	advancedBackup  string
}

// ensureAdminOnlyFieldsUnchanged rejects a non-admin write that differs from the
// stored host in any admin-only field. An unchanged value is always accepted.
func ensureAdminOnlyFieldsUnchanged(admin bool, stored, next hostAdminFields) error {
	if admin {
		return nil
	}
	if !sameUintPointer(stored.dnsProviderID, next.dnsProviderID) ||
		stored.useDNSChallenge != next.useDNSChallenge ||
		stored.advancedConfig != next.advancedConfig ||
		stored.advancedBackup != next.advancedBackup {
		return ErrAdminOnlyHostField
	}
	return nil
}

func sameUintPointer(a, b *uint) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// storedProxyAdminFields loads the admin-only fields of a stored proxy host. A
// zero id (a create) or a missing row yields empty fields. The advanced config
// is compared in canonical form so that stored rows written before
// normalization was applied do not block unrelated non-admin writes.
func storedProxyAdminFields(tx *gorm.DB, id uint) (hostAdminFields, error) {
	if id == 0 {
		return hostAdminFields{}, nil
	}
	var stored models.ProxyHost
	err := tx.Select("dns_provider_id", "use_dns_challenge", "advanced_config", "advanced_config_backup").
		Where("id = ?", id).First(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return hostAdminFields{}, nil
	}
	if err != nil {
		return hostAdminFields{}, fmt.Errorf("load stored proxy host: %w", err)
	}
	return hostAdminFields{
		dnsProviderID:   stored.DNSProviderID,
		useDNSChallenge: stored.UseDNSChallenge,
		advancedConfig:  canonicalAdvancedConfig(stored.AdvancedConfig),
		advancedBackup:  stored.AdvancedConfigBackup,
	}, nil
}

// storedRedirectionAdminFields loads the admin-only fields of a stored redirection
// host. Redirection hosts have no advanced configuration.
func storedRedirectionAdminFields(tx *gorm.DB, id uint) (hostAdminFields, error) {
	if id == 0 {
		return hostAdminFields{}, nil
	}
	var stored models.RedirectionHost
	err := tx.Select("dns_provider_id", "use_dns_challenge").Where("id = ?", id).First(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return hostAdminFields{}, nil
	}
	if err != nil {
		return hostAdminFields{}, fmt.Errorf("load stored redirection host: %w", err)
	}
	return hostAdminFields{dnsProviderID: stored.DNSProviderID, useDNSChallenge: stored.UseDNSChallenge}, nil
}

// canonicalAdvancedConfig returns the normalized form of a stored advanced config.
// A value that cannot be normalized is returned unchanged.
func canonicalAdvancedConfig(raw string) string {
	if raw == "" {
		return ""
	}
	norm, err := normalizeAdvancedJSON(raw)
	if err != nil {
		return raw
	}
	return norm
}

// adminAPIPortFrom extracts the port of a host:port listen address. It returns 0
// for an address without a valid port, which matches no forward target.
func adminAPIPortFrom(address string) int {
	_, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}

// forwardTargetHost returns the host part of a forward target, with any scheme,
// port and path removed.
func forwardTargetHost(raw string) string {
	target := strings.TrimSpace(raw)
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		if u, err := url.Parse(target); err == nil {
			target = u.Hostname()
		} else {
			target = strings.TrimPrefix(target, "http://")
			target = strings.TrimPrefix(target, "https://")
		}
	}
	if h, _, err := net.SplitHostPort(target); err == nil {
		target = h
	}
	if idx := strings.Index(target, "/"); idx != -1 {
		target = target[:idx]
	}
	return target
}

// refuseAdminListenerTarget rejects a forward target that reaches the proxy's own
// admin listener: a local name or address on the admin port. It is shared by proxy
// host and location validation.
func refuseAdminListenerTarget(forwardHost string, port int) error {
	if caddyAdminAPIPort == 0 || port != caddyAdminAPIPort {
		return nil
	}
	if localHostChecker.IsLocalHost(forwardTargetHost(forwardHost)) {
		return ErrAdminListenerTarget
	}
	return nil
}
