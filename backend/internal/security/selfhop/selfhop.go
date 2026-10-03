// Package selfhop provides the building blocks Charon uses to recognise
// requests that reach its own API through its embedded reverse proxy.
//
// The proxy attaches a per-process secret and the connecting client address to
// requests whose upstream is Charon itself. The API verifies the secret in
// constant time before it relies on the address. The secret lives in memory
// only and is redacted by every formatting path.
package selfhop

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// HeaderSecret carries the per-process secret.
	HeaderSecret = "X-Charon-Self-Hop" //nolint:gosec // header name, not a credential
	// HeaderClient carries the address of the client connected to the proxy.
	HeaderClient = "X-Charon-Self-Hop-Client"
	// ClientPlaceholder is the Caddy placeholder that expands to the address of
	// the TCP peer connected to the proxy.
	ClientPlaceholder = "{http.request.remote.host}"

	secretBytes     = 32
	minSecretLength = 32
	redacted        = "[redacted]"
)

// randRead is the entropy source; replaced in tests.
var randRead = rand.Read

// Secret is an opaque per-process shared secret.
type Secret struct {
	value string
}

// NewSecret generates a fresh random secret.
func NewSecret() (*Secret, error) {
	buf := make([]byte, secretBytes)
	if _, err := randRead(buf); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	return &Secret{value: hex.EncodeToString(buf)}, nil
}

// ParseSecret wraps an existing value, rejecting values that are too short.
func ParseSecret(value string) (*Secret, error) {
	if len(value) < minSecretLength {
		return nil, errors.New("secret too short")
	}
	return &Secret{value: value}, nil
}

// Reveal returns the raw value. It is only meant for building proxy configuration.
func (s *Secret) Reveal() string {
	if s == nil {
		return ""
	}
	return s.value
}

// Verify reports whether candidate equals the secret, in constant time.
// A nil or empty secret never verifies.
func (s *Secret) Verify(candidate string) bool {
	if s == nil || s.value == "" || candidate == "" {
		return false
	}
	want := sha256.Sum256([]byte(s.value))
	got := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

// String implements fmt.Stringer without exposing the value.
func (s *Secret) String() string { return redacted }

// GoString implements fmt.GoStringer without exposing the value.
func (s *Secret) GoString() string { return redacted }

// Format redacts the value for every fmt verb.
func (s *Secret) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(redacted)) }

// MarshalText redacts the value for encoders.
func (s *Secret) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// LocalAddrs answers whether an address belongs to this host: loopback or the
// address of one of its network interfaces. The interface list is cached for a
// short time because it rarely changes.
type LocalAddrs struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	lister  func() ([]net.Addr, error)
	fetched time.Time
	ips     []net.IP
}

// defaultLocalAddrsTTL bounds how stale the cached interface list may be.
const defaultLocalAddrsTTL = 30 * time.Second

// NewLocalAddrs returns a LocalAddrs backed by the host's interfaces.
func NewLocalAddrs() *LocalAddrs {
	return &LocalAddrs{ttl: defaultLocalAddrsTTL, now: time.Now, lister: net.InterfaceAddrs}
}

func (l *LocalAddrs) snapshot() []net.IP {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if l.ips != nil && now.Sub(l.fetched) < l.ttl {
		return l.ips
	}
	addrs, err := l.lister()
	if err != nil {
		// Keep serving the previous snapshot rather than failing open.
		return l.ips
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		switch v := a.(type) {
		case *net.IPNet:
			ips = append(ips, v.IP)
		case *net.IPAddr:
			ips = append(ips, v.IP)
		}
	}
	l.ips, l.fetched = ips, now
	return ips
}

// Contains reports whether ip is loopback or an interface address of this host.
func (l *LocalAddrs) Contains(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	for _, local := range l.snapshot() {
		if local.Equal(ip) {
			return true
		}
	}
	return false
}

// IsLocalHost reports whether a dial host names this machine: a loopback or
// unspecified address, an interface address, "localhost" or the host name.
func (l *LocalAddrs) IsLocalHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(strings.TrimSpace(host), "[]"), "."))
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsUnspecified() || l.Contains(ip)
	}
	for _, name := range hostNames() {
		if host == name {
			return true
		}
	}
	return false
}

func hostNames() []string {
	var names []string
	if h, err := os.Hostname(); err == nil && h != "" {
		names = append(names, strings.ToLower(h))
	}
	if h := os.Getenv("HOSTNAME"); h != "" {
		names = append(names, strings.ToLower(h))
	}
	return names
}
