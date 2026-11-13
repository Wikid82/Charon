package config

import (
	"errors"
	"fmt"
	neturl "net/url"
	"path/filepath"
	"strings"

	"github.com/Wikid82/charon/backend/internal/security"
)

// CaddyAdminTCPAddress is the loopback TCP listener the bundled Caddy uses only
// when CHARON_CADDY_ADMIN_API names a loopback URL (development and test setups).
// The default configuration never opens it.
const CaddyAdminTCPAddress = "127.0.0.1:2019"

// CaddyAdminTCPPort is the port of CaddyAdminTCPAddress. The service layer also
// refuses forward targets on this port.
const CaddyAdminTCPPort = 2019

// DefaultCaddyAdminSocket is the default admin socket path, relative to the
// working directory (/app in the container). It sits in a directory only the
// charon user can enter.
const DefaultCaddyAdminSocket = "data/run/caddy-admin.sock"

// caddyAdminSocketPrefix marks a CHARON_CADDY_ADMIN_API value that names a unix socket.
const caddyAdminSocketPrefix = "unix://"

// maxUnixSocketPath is the longest socket path the kernel accepts (sun_path holds
// 108 bytes including the terminating NUL).
const maxUnixSocketPath = 107

// caddyAdminLoopbackHosts are the host names that reach this container's loopback interface.
var caddyAdminLoopbackHosts = map[string]struct{}{
	"localhost": {},
	"127.0.0.1": {},
	"::1":       {},
}

// caddyAdminRefusedMessage is the startup error for an admin setting that names
// neither a socket path nor this container's loopback address.
const caddyAdminRefusedMessage = "CHARON_CADDY_ADMIN_API must be a unix:// socket path or point at this container (localhost); using an external proxy is not supported: remove the setting to use the default socket"

// DefaultCaddyAdminAPI returns the admin API setting used when none is configured:
// the default socket path made absolute. If the working directory cannot be
// resolved, it returns the relative form.
func DefaultCaddyAdminAPI() string {
	abs, err := filepath.Abs(DefaultCaddyAdminSocket)
	if err != nil {
		return caddyAdminSocketPrefix + DefaultCaddyAdminSocket
	}
	return caddyAdminSocketPrefix + abs
}

// CaddyAdminSocketPath returns the socket path when adminAPI selects the unix socket.
func CaddyAdminSocketPath(adminAPI string) (string, bool) {
	if !strings.HasPrefix(adminAPI, caddyAdminSocketPrefix) {
		return "", false
	}
	return strings.TrimPrefix(adminAPI, caddyAdminSocketPrefix), true
}

// CaddyAdminListen returns the admin.listen value that makes Caddy serve the
// admin API on the endpoint adminAPI names. The socket is created with mode 0600,
// so only its owner can connect; the entrypoint makes its directory 0700.
func CaddyAdminListen(adminAPI string) string {
	if path, ok := CaddyAdminSocketPath(adminAPI); ok {
		// Caddy's unix form is "unix/" followed by the absolute path: "unix//abs/path|0600".
		return "unix/" + path + "|0600"
	}
	return CaddyAdminTCPAddress
}

// resolveCaddyAdminAPI validates CHARON_CADDY_ADMIN_API and returns the endpoint
// Charon uses. A unix:// value is kept, with an absolute path. A loopback URL on the
// admin port is accepted for compatibility with earlier configurations. Charon
// ignores it and uses the default socket, so the container never opens a TCP admin
// listener, and legacy reports that case so the caller can warn. The host check
// runs before the generic validator, so a service name is refused even when it
// appears in the internal-service allowlist.
func resolveCaddyAdminAPI(raw string) (endpoint string, legacy bool, err error) {
	if strings.HasPrefix(raw, caddyAdminSocketPrefix) {
		path, pathErr := absoluteSocketPath(strings.TrimPrefix(raw, caddyAdminSocketPrefix))
		if pathErr != nil {
			return "", false, fmt.Errorf("invalid caddy admin socket path: %w", pathErr)
		}
		return caddyAdminSocketPrefix + path, false, nil
	}
	u, err := neturl.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("invalid url format: %w", err)
	}
	if _, ok := caddyAdminLoopbackHosts[strings.ToLower(u.Hostname())]; !ok {
		return "", false, errors.New(caddyAdminRefusedMessage)
	}
	if _, err := security.ValidateInternalServiceBaseURL(raw, CaddyAdminTCPPort, caddyAdminLoopbackHosts); err != nil {
		return "", false, fmt.Errorf("invalid caddy admin api url: %w", err)
	}
	return DefaultCaddyAdminAPI(), true, nil
}

// LegacyAdminURLWarning returns the startup warning for a loopback URL setting that
// Charon no longer uses. socketEndpoint is the endpoint Charon uses instead.
func LegacyAdminURLWarning(socketEndpoint string) string {
	return "CHARON_CADDY_ADMIN_API names a loopback URL, which Charon no longer uses: the Caddy admin API is the unix socket " +
		strings.TrimPrefix(socketEndpoint, caddyAdminSocketPrefix) + "; remove the setting"
}

// absoluteSocketPath returns the cleaned absolute form of a socket path. It refuses
// characters that Caddy's listen syntax or the entrypoint's JSON cannot carry.
func absoluteSocketPath(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("socket path is empty")
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve socket path: %w", err)
	}
	if strings.ContainsAny(abs, "|\"\\") {
		return "", errors.New("socket path must not contain '|', '\"' or '\\'")
	}
	for _, r := range abs {
		if r < 0x20 || r == 0x7f {
			return "", errors.New("socket path must not contain control characters")
		}
	}
	if len(abs) > maxUnixSocketPath {
		return "", fmt.Errorf("socket path is longer than %d bytes: set CHARON_CADDY_ADMIN_API to a unix:// path that is shorter", maxUnixSocketPath)
	}
	return abs, nil
}
