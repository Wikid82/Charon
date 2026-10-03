package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

func testHopSecret(t *testing.T) *selfhop.Secret {
	t.Helper()
	s, err := selfhop.NewSecret()
	require.NoError(t, err)
	return s
}

// proxyUse is one reverse_proxy handler found in a generated config.
type proxyUse struct {
	dial string
	set  map[string][]string
}

func collectProxies(t *testing.T, cfg *Config) []proxyUse {
	t.Helper()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	var doc any
	require.NoError(t, json.Unmarshal(raw, &doc))

	var out []proxyUse
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if x["handler"] == "reverse_proxy" {
				use := proxyUse{set: map[string][]string{}}
				if ups, ok := x["upstreams"].([]any); ok && len(ups) > 0 {
					use.dial, _ = ups[0].(map[string]any)["dial"].(string)
				}
				if hdrs, ok := x["headers"].(map[string]any); ok {
					if req, ok := hdrs["request"].(map[string]any); ok {
						if set, ok := req["set"].(map[string]any); ok {
							for k, vals := range set {
								for _, val := range vals.([]any) {
									use.set[k] = append(use.set[k], val.(string))
								}
							}
						}
					}
				}
				out = append(out, use)
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(doc)
	return out
}

func generateWithOpts(t *testing.T, hosts []models.ProxyHost, opts ...GenerateConfigOption) *Config {
	t.Helper()
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil, opts...)
	require.NoError(t, err)
	return cfg
}

func fixtureHosts() []models.ProxyHost {
	disabled := false
	return []models.ProxyHost{
		{
			UUID: "u-self", DomainNames: "charon.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			EnableStandardHeaders: &disabled,
			Locations: []models.Location{
				{Path: "/api-local", ForwardHost: "localhost", ForwardPort: 8080},
				{Path: "/other", ForwardHost: "10.0.0.9", ForwardPort: 9000},
			},
		},
		{UUID: "u-third", DomainNames: "app.example.com", ForwardHost: "10.0.0.5", ForwardPort: 3000, Enabled: true},
		{UUID: "u-sameip-otherport", DomainNames: "other.example.com", ForwardHost: "127.0.0.1", ForwardPort: 9090, Enabled: true},
		{UUID: "u-remote", DomainNames: "remote.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true},
	}
}

func TestGenerateConfig_SelfHopHeadersOnlyOnSelfUpstream(t *testing.T) {
	secret := testHopSecret(t)
	cfg := generateWithOpts(t, fixtureHosts(),
		WithSelfHop(secret, "8080"),
		WithRemoteHosts(map[string]struct{}{"u-remote": {}}),
	)

	var selfCount, otherCount int
	for _, p := range collectProxies(t, cfg) {
		_, hasSecret := p.set[selfhop.HeaderSecret]
		switch p.dial {
		case "127.0.0.1:8080", "localhost:8080":
			// u-self (main + emergency + location) and u-remote (never self).
			if hasSecret {
				selfCount++
				assert.Equal(t, []string{secret.Reveal()}, p.set[selfhop.HeaderSecret])
				assert.Equal(t, []string{selfhop.ClientPlaceholder}, p.set[selfhop.HeaderClient])
				assert.Equal(t, []string{"{http.request.scheme}"}, p.set["X-Forwarded-Proto"])
				assert.Equal(t, []string{"{http.request.host}"}, p.set["X-Forwarded-Host"])
				assert.NotContains(t, p.set, "Add")
			}
		default:
			otherCount++
			assert.False(t, hasSecret, "dial %s must not carry hop headers", p.dial)
			assert.NotContains(t, p.set, selfhop.HeaderClient)
		}
	}
	// main route, emergency route and one self location of u-self.
	assert.Equal(t, 3, selfCount)
	assert.Positive(t, otherCount)

	// The remote host dials the same address:port but never receives proof.
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	assert.Equal(t, 3, strings.Count(string(raw), secret.Reveal()))
}

func TestGenerateConfig_WithoutSelfHopIsUnchanged(t *testing.T) {
	baseline := generateWithOpts(t, fixtureHosts())
	withRemote := generateWithOpts(t, fixtureHosts(), WithRemoteHosts(map[string]struct{}{"u-remote": {}}))

	a, err := json.Marshal(baseline)
	require.NoError(t, err)
	b, err := json.Marshal(withRemote)
	require.NoError(t, err)
	assert.Equal(t, string(a), string(b))
	assert.NotContains(t, string(a), selfhop.HeaderSecret)
	assert.NotContains(t, string(a), selfhop.HeaderClient)
}

func TestGenerateConfig_NonSelfRoutesIdenticalWithSelfHopEnabled(t *testing.T) {
	nonSelf := []models.ProxyHost{
		{UUID: "u-third", DomainNames: "app.example.com", ForwardHost: "10.0.0.5", ForwardPort: 3000, Enabled: true},
		{UUID: "u-port", DomainNames: "other.example.com", ForwardHost: "127.0.0.1", ForwardPort: 9090, Enabled: true},
		{UUID: "u-name", DomainNames: "named.example.com", ForwardHost: "media", ForwardPort: 8080, Enabled: true},
	}
	a, err := json.Marshal(generateWithOpts(t, nonSelf))
	require.NoError(t, err)
	b, err := json.Marshal(generateWithOpts(t, nonSelf, WithSelfHop(testHopSecret(t), "8080")))
	require.NoError(t, err)
	assert.Equal(t, string(a), string(b))
}

func TestSelfHopConfig_IsSelfUpstream(t *testing.T) {
	t.Setenv("HOSTNAME", "charon-box")
	local := selfhop.NewLocalAddrs()
	cfg := &selfHopConfig{secret: testHopSecret(t), port: "8080", local: local}

	tests := []struct {
		dial   string
		remote bool
		want   bool
	}{
		{"127.0.0.1:8080", false, true},
		{"localhost:8080", false, true},
		{"[::1]:8080", false, true},
		{"0.0.0.0:8080", false, true},
		{"charon-box:8080", false, true},
		{"127.0.0.1:08080", false, true},
		{"127.0.0.1:8081", false, false},
		{"127.0.0.1:80", false, false},
		{"10.99.99.99:8080", false, false},
		{"example.com:8080", false, false},
		{"127.0.0.1:8080", true, false},
		{"127.0.0.1", false, false},
		{"127.0.0.1:abc", false, false},
		{"", false, false},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, cfg.isSelfUpstream(tc.dial, tc.remote), "dial=%q remote=%v", tc.dial, tc.remote)
	}

	var nilCfg *selfHopConfig
	assert.False(t, nilCfg.isSelfUpstream("127.0.0.1:8080", false))
	assert.False(t, (&selfHopConfig{port: "8080", local: local}).isSelfUpstream("127.0.0.1:8080", false))
}

func TestSelfHopConfig_OwnInterfaceAddressIsSelf(t *testing.T) {
	addrs, err := net.InterfaceAddrs()
	require.NoError(t, err)
	var own string
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() {
			own = ipn.IP.String()
			break
		}
	}
	if own == "" {
		t.Skip("no non-loopback IPv4 interface address")
	}
	cfg := &selfHopConfig{secret: testHopSecret(t), port: "8080", local: selfhop.NewLocalAddrs()}
	assert.True(t, cfg.isSelfUpstream(own+":8080", false))
}

func TestOrthrusHostUUIDs(t *testing.T) {
	assert.Nil(t, orthrusHostUUIDs([]models.ProxyHost{{UUID: "a", ForwardHost: "10.0.0.1"}}))
	got := orthrusHostUUIDs([]models.ProxyHost{
		{UUID: "a", ForwardHost: "orthrus:agent-1"},
		{UUID: "b", ForwardHost: "10.0.0.1"},
	})
	assert.Equal(t, map[string]struct{}{"a": {}}, got)
}

type captureClient struct {
	loaded []*Config
}

func (c *captureClient) Load(_ context.Context, cfg *Config) error {
	c.loaded = append(c.loaded, cfg)
	return nil
}
func (c *captureClient) Ping(context.Context) error                 { return nil }
func (c *captureClient) GetConfig(context.Context) (*Config, error) { return &Config{}, nil }

type stubOrthrus struct{ addr string }

func (s stubOrthrus) GetProxyAddr(string) (string, bool) { return s.addr, true }

func newHopManager(t *testing.T) (*Manager, *captureClient, *gorm.DB, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))
	dir := t.TempDir()
	client := &captureClient{}
	return NewManager(client, db, dir, "", false, config.SecurityConfig{}), client, db, dir
}

func TestManager_ApplyConfig_SelfHopWiringAndSnapshotRedaction(t *testing.T) {
	m, client, db, dir := newHopManager(t)
	secret := testHopSecret(t)
	m.SetSelfHop(secret, "8080")
	m.SetOrthrusServer(stubOrthrus{addr: "127.0.0.1:8080"})
	require.NoError(t, db.Create(&models.ProxyHost{UUID: "u-self", DomainNames: "charon.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}).Error)
	require.NoError(t, db.Create(&models.ProxyHost{UUID: "u-agent", DomainNames: "agent.example.com", ForwardHost: "orthrus:agent-1", ForwardPort: 1, Enabled: true}).Error)

	require.NoError(t, m.ApplyConfig(context.Background()))
	require.Len(t, client.loaded, 1)

	// Live config: the self route carries the proof, the agent route (resolved to
	// the same address) does not.
	withProof, without := 0, 0
	for _, p := range collectProxies(t, client.loaded[0]) {
		if p.dial != "127.0.0.1:8080" {
			continue
		}
		if len(p.set[selfhop.HeaderSecret]) == 1 && p.set[selfhop.HeaderSecret][0] == secret.Reveal() {
			withProof++
		} else {
			without++
		}
	}
	assert.Equal(t, 2, withProof) // main + emergency route of the self host
	assert.Equal(t, 2, without)   // main + emergency route of the agent host

	// Snapshots on disk never hold the secret.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	snapshots := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		snapshots++
		body, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, readErr)
		assert.NotContains(t, string(body), secret.Reveal())
		assert.Contains(t, string(body), snapshotSecretPlaceholder)
	}
	assert.Equal(t, 1, snapshots)

	// Rolling back restores the live secret.
	require.NoError(t, m.rollback(context.Background()))
	require.Len(t, client.loaded, 2)
	raw, err := json.Marshal(client.loaded[1])
	require.NoError(t, err)
	assert.Contains(t, string(raw), secret.Reveal())
	assert.NotContains(t, string(raw), snapshotSecretPlaceholder)
}

func TestManager_ApplyConfig_WithoutSelfHopAddsNoProof(t *testing.T) {
	m, client, db, _ := newHopManager(t)
	require.NoError(t, db.Create(&models.ProxyHost{UUID: "u-self", DomainNames: "charon.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}).Error)

	require.NoError(t, m.ApplyConfig(context.Background()))
	raw, err := json.Marshal(client.loaded[0])
	require.NoError(t, err)
	assert.NotContains(t, string(raw), selfhop.HeaderSecret)

	// Redaction helpers are no-ops without a secret.
	assert.Equal(t, []byte("x"), m.redactHopSecret([]byte("x")))
	assert.Equal(t, []byte(snapshotSecretPlaceholder), m.restoreHopSecret([]byte(snapshotSecretPlaceholder)))
}
