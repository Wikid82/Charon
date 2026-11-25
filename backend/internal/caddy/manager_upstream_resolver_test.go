package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// rawCaptureClient records the encoded bodies sent to /load, as the real client does.
type rawCaptureClient struct {
	captureClient
	bodies [][]byte
}

func (c *rawCaptureClient) LoadJSON(_ context.Context, body []byte) error {
	c.bodies = append(c.bodies, body)
	return nil
}

// portGuard reports a fixed reserved port set, or an error.
type portGuard struct {
	ports map[int]struct{}
	err   error
}

func (g portGuard) RefusesForwardTarget(string, int) (bool, error) { return false, nil }

func (g portGuard) ReservedPorts() (map[int]struct{}, error) { return g.ports, g.err }

var testResolver = UpstreamResolver{General: "udp/127.0.0.1:41000", Reserved: "udp/127.0.0.1:41001"}

// decodeNumbers decodes a JSON document keeping numbers exact.
func decodeNumbers(t *testing.T, raw []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	require.NoError(t, dec.Decode(&doc))
	return doc
}

// reverseProxyObjects returns every reverse_proxy object of a decoded document.
func reverseProxyObjects(t *testing.T, doc any) []map[string]any {
	t.Helper()
	var out []map[string]any
	require.NoError(t, walkObjects(doc, func(obj map[string]any) error {
		if obj["handler"] == "reverse_proxy" {
			out = append(out, obj)
		}
		return nil
	}))
	return out
}

// resolverAddresses returns the resolver addresses of a reverse_proxy object.
func resolverAddresses(t *testing.T, handler map[string]any) []any {
	t.Helper()
	transport, ok := handler["transport"].(map[string]any)
	require.True(t, ok, "transport is set")
	resolver, ok := transport["resolver"].(map[string]any)
	require.True(t, ok, "transport.resolver is set")
	addrs, ok := resolver["addresses"].([]any)
	require.True(t, ok)
	return addrs
}

// stripTransports removes every transport key from the reverse_proxy objects of a
// decoded document, so an injected copy can be compared with the original.
func stripTransports(t *testing.T, doc any) any {
	t.Helper()
	for _, obj := range reverseProxyObjects(t, doc) {
		delete(obj, "transport")
	}
	return doc
}

func TestManagerLoad_NoResolverPassesTheConfigUnchanged(t *testing.T) {
	m, client, _, _ := newHopManager(t)
	conf := configWithUpstream("app.lan.test:8080")
	require.NoError(t, m.load(context.Background(), conf))
	require.Len(t, client.loaded, 1)
	assert.Same(t, conf, client.loaded[0], "without a resolver the same config reaches the client")
}

func TestManagerLoad_InjectsResolverWithoutChangingTheConfig(t *testing.T) {
	m, _, _, _ := newHopManager(t)
	client := &rawCaptureClient{}
	m.client = client
	m.SetUpstreamResolver(testResolver)
	conf := configWithUpstream("app.lan.test:8080")
	before, err := json.Marshal(conf)
	require.NoError(t, err)

	require.NoError(t, m.load(context.Background(), conf))
	after, err := json.Marshal(conf)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the caller's config is not modified")

	require.Len(t, client.bodies, 1)
	doc := decodeNumbers(t, client.bodies[0])
	handlers := reverseProxyObjects(t, doc)
	require.Len(t, handlers, 1)
	assert.Equal(t, []any{"udp/127.0.0.1:41000"}, resolverAddresses(t, handlers[0]))
	assert.Equal(t, "http", handlers[0]["transport"].(map[string]any)["protocol"])
}

func TestManagerLoad_FidelityKeepsEveryOtherValue(t *testing.T) {
	m, _, _, _ := newHopManager(t)
	client := &rawCaptureClient{}
	m.client = client
	m.SetUpstreamResolver(testResolver)
	conf := &Config{Apps: Apps{HTTP: &HTTPApp{Servers: map[string]*Server{
		"s": {Listen: []string{":443"}, Routes: []*Route{{Handle: []Handler{
			{"handler": "reverse_proxy", "upstreams": []map[string]any{{"dial": "app.lan.test:8080"}},
				"health_checks": map[string]any{"active": map[string]any{"uri": "/health", "port": 8080}}},
			{"handler": "reverse_proxy", "upstreams": []map[string]any{{"dial": "10.0.0.9:9000"}},
				"transport": map[string]any{"protocol": "http", "tls": map[string]any{}, "read_timeout": 30000000000}},
		}}}},
	}}}}
	require.NoError(t, m.load(context.Background(), conf))

	raw, err := json.Marshal(conf)
	require.NoError(t, err)
	original := decodeNumbers(t, raw)
	injected := decodeNumbers(t, client.bodies[0])
	for _, handler := range reverseProxyObjects(t, injected) {
		assert.NotEmpty(t, resolverAddresses(t, handler))
	}
	assert.Equal(t, stripTransports(t, original), stripTransports(t, injected),
		"only transport keys differ between the loaded and the original document")
	assert.Contains(t, string(client.bodies[0]), `"read_timeout":30000000000`, "large numbers keep their exact digits")
}

func TestManagerLoad_ReservedPortUsesTheReservedListener(t *testing.T) {
	m, _, _, _ := newHopManager(t)
	client := &rawCaptureClient{}
	m.client = client
	m.SetUpstreamResolver(testResolver)
	m.forwardGuard = portGuard{ports: map[int]struct{}{8085: {}}}
	conf := &Config{Apps: Apps{HTTP: &HTTPApp{Servers: map[string]*Server{
		"s": {Listen: []string{":443"}, Routes: []*Route{{Handle: []Handler{
			ReverseProxyHandler("app.lan.test:8080", "", true),
			ReverseProxyHandler("app.lan.test:8085", "", true),
		}}}},
	}}}}
	require.NoError(t, m.load(context.Background(), conf))

	handlers := reverseProxyObjects(t, decodeNumbers(t, client.bodies[0]))
	require.Len(t, handlers, 2)
	assert.Equal(t, []any{"udp/127.0.0.1:41000"}, resolverAddresses(t, handlers[0]))
	assert.Equal(t, []any{"udp/127.0.0.1:41001"}, resolverAddresses(t, handlers[1]))
}

func TestManagerLoad_ReservedPortReadErrorFailsTheLoad(t *testing.T) {
	m, client, _, _ := newHopManager(t)
	m.SetUpstreamResolver(testResolver)
	m.forwardGuard = portGuard{err: errors.New("store down")}

	err := m.load(context.Background(), configWithUpstream("app.lan.test:8080"))
	require.ErrorContains(t, err, "read reserved ports")
	assert.Empty(t, client.loaded)
}

func TestManagerLoad_UnusableTransportWithNameFailsTheLoad(t *testing.T) {
	m, client, _, _ := newHopManager(t)
	m.SetUpstreamResolver(testResolver)
	conf := &Config{Apps: Apps{HTTP: &HTTPApp{Servers: map[string]*Server{
		"s": {Listen: []string{":443"}, Routes: []*Route{{Handle: []Handler{{
			"handler":   "reverse_proxy",
			"transport": map[string]any{"protocol": "fastcgi"},
			"upstreams": []map[string]any{{"dial": "app.lan.test:9000"}},
		}}}}},
	}}}}
	err := m.load(context.Background(), conf)
	require.ErrorIs(t, err, ErrUpstreamResolverUnusable)
	assert.Empty(t, client.loaded)
}

func TestManagerLoad_InjectedConfigPassesValidate(t *testing.T) {
	m, _, _, _ := newHopManager(t)
	client := &rawCaptureClient{}
	m.client = client
	m.SetUpstreamResolver(testResolver)
	conf := configWithUpstream("app.lan.test:8080")
	conf.Apps.HTTP.Servers["s"].Listen = []string{":443"}
	require.NoError(t, Validate(conf))
	require.NoError(t, m.load(context.Background(), conf))

	var injected Config
	require.NoError(t, json.Unmarshal(client.bodies[0], &injected))
	require.NoError(t, Validate(&injected), "decoded injected bytes validate")
}

func TestManagerLoad_TopLevelAdvancedReverseProxyValidates(t *testing.T) {
	// A host whose advanced configuration is a top-level reverse_proxy: its upstreams are []any after decoding.
	raw := `{"apps":{"http":{"servers":{"srv0":{"listen":[":443"],"routes":[` +
		`{"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"app.lan.test:8080"}]}]}` +
		`]}}}}}`
	m, _, _, _ := newHopManager(t)
	client := &rawCaptureClient{}
	m.client = client
	m.SetUpstreamResolver(testResolver)
	var conf Config
	require.NoError(t, json.Unmarshal([]byte(raw), &conf))
	require.NoError(t, Validate(&conf))
	require.NoError(t, m.load(context.Background(), &conf))
	var injected Config
	require.NoError(t, json.Unmarshal(client.bodies[0], &injected))
	require.NoError(t, Validate(&injected))
}

func TestManagerApplyConfig_SnapshotAndHashHoldNoResolver(t *testing.T) {
	m, _, db, dir := newHopManager(t)
	client := &rawCaptureClient{}
	m.client = client
	m.SetUpstreamResolver(testResolver)
	require.NoError(t, db.Create(&models.ProxyHost{UUID: "u-name", DomainNames: "name.example.com", ForwardHost: "app.lan.test", ForwardPort: 8080, Enabled: true}).Error)

	require.NoError(t, m.ApplyConfig(context.Background()))
	require.Len(t, client.bodies, 1)
	assert.Contains(t, string(client.bodies[0]), "udp/127.0.0.1:41000", "the loaded config names the resolver")

	snapshots, err := filepath.Glob(filepath.Join(dir, "config-*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, snapshots)
	for _, path := range snapshots {
		raw, readErr := os.ReadFile(filepath.Clean(path))
		require.NoError(t, readErr)
		assert.NotContains(t, string(raw), "resolver", "snapshots never carry a resolver")
		assert.NotContains(t, string(raw), "udp/127.0.0.1", "snapshots never carry a resolver address")
	}
}

func TestRollback_OldSnapshotIsLoadedWithTheResolver(t *testing.T) {
	m, _, _, _ := newHopManager(t)
	client := &rawCaptureClient{}
	m.client = client
	m.SetUpstreamResolver(testResolver)
	_, err := m.saveSnapshot(configWithUpstream("app.lan.test:8080"))
	require.NoError(t, err)

	require.NoError(t, m.rollback(context.Background()))
	require.Len(t, client.bodies, 1)
	assert.Equal(t, []any{"udp/127.0.0.1:41000"}, resolverAddresses(t, reverseProxyObjects(t, decodeNumbers(t, client.bodies[0]))[0]))
}

func TestRollback_SnapshotWithUnusableTransportIsNotReinstated(t *testing.T) {
	m, client, _, _ := newHopManager(t)
	m.SetUpstreamResolver(testResolver)
	conf := &Config{Apps: Apps{HTTP: &HTTPApp{Servers: map[string]*Server{
		"s": {Listen: []string{":443"}, Routes: []*Route{{Handle: []Handler{{
			"handler":   "reverse_proxy",
			"transport": map[string]any{"protocol": "fastcgi"},
			"upstreams": []map[string]any{{"dial": "app.lan.test:9000"}},
		}}}}},
	}}}}
	_, err := m.saveSnapshot(conf)
	require.NoError(t, err)

	err = m.rollback(context.Background())
	require.ErrorIs(t, err, errSnapshotRefusedTarget)
	assert.Empty(t, client.loaded)
}

func TestManager_UpstreamNameChecksAvailable(t *testing.T) {
	m, _, _, _ := newHopManager(t)
	assert.False(t, m.UpstreamNameChecksAvailable())
	m.SetUpstreamResolver(testResolver)
	assert.True(t, m.UpstreamNameChecksAvailable())
}

func TestApplyUpstreamResolver_Rules(t *testing.T) {
	t.Parallel()
	reserved := map[int]struct{}{8085: {}}
	tests := map[string]struct {
		raw         string
		wantAddress string
		wantErr     error
	}{
		"missing transport gets the default http transport": {
			raw: `{"handler":"reverse_proxy","upstreams":[{"dial":"app.lan:8080"}]}`, wantAddress: "udp/127.0.0.1:41000",
		},
		"existing http transport keeps its other keys": {
			raw: `{"handler":"reverse_proxy","transport":{"protocol":"http","tls":{}},"upstreams":[{"dial":"app.lan:8080"}]}`, wantAddress: "udp/127.0.0.1:41000",
		},
		"reserved dial port uses the reserved listener": {
			raw: `{"handler":"reverse_proxy","upstreams":[{"dial":"app.lan:8085"}]}`, wantAddress: "udp/127.0.0.1:41001",
		},
		"port range overlapping a reserved port is reserved": {
			raw: `{"handler":"reverse_proxy","upstreams":[{"dial":"app.lan:8080-8090"}]}`, wantAddress: "udp/127.0.0.1:41001",
		},
		"reserved health check port uses the reserved listener": {
			raw: `{"handler":"reverse_proxy","upstreams":[{"dial":"10.0.0.9:8080"}],"health_checks":{"active":{"port":8085}}}`, wantAddress: "udp/127.0.0.1:41001",
		},
		"health check upstream on a reserved port uses the reserved listener": {
			raw: `{"handler":"reverse_proxy","upstreams":[{"dial":"10.0.0.9:8080"}],"health_checks":{"active":{"upstream":"probe.lan:8085"}}}`, wantAddress: "udp/127.0.0.1:41001",
		},
		"unparsable dial uses the reserved listener": {
			raw: `{"handler":"reverse_proxy","upstreams":[{"dial":"{http.request.header.X}:80"}]}`, wantAddress: "udp/127.0.0.1:41001",
		},
		"literal target with an unusable transport is left alone": {
			raw: `{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"10.0.0.9:9000"}]}`,
		},
		"name target with an unusable transport is refused": {
			raw: `{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"app.lan:9000"}]}`, wantErr: ErrUpstreamResolverUnusable,
		},
		"name in a health check with an unusable transport is refused": {
			raw: `{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"10.0.0.9:9000"}],"health_checks":{"active":{"upstream":"probe.lan:9000"}}}`, wantErr: ErrUpstreamResolverUnusable,
		},
		"non reverse proxy objects are not touched": {
			raw: `{"handler":"file_server","root":"/srv"}`,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			doc := decodeNumbers(t, []byte(tc.raw))
			err := ApplyUpstreamResolver(doc, testResolver, reserved)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			obj := doc.(map[string]any)
			if tc.wantAddress == "" {
				if transport, ok := obj["transport"].(map[string]any); ok {
					assert.NotContains(t, transport, "resolver", "a literal target is left as it is")
				}
				return
			}
			if obj["handler"] != "reverse_proxy" {
				return
			}
			assert.Equal(t, []any{tc.wantAddress}, resolverAddresses(t, obj))
		})
	}
}

func TestApplyUpstreamResolver_KeepsHttpTransportFields(t *testing.T) {
	t.Parallel()
	doc := decodeNumbers(t, []byte(`{"handler":"reverse_proxy","transport":{"protocol":"http","tls":{"insecure_skip_verify":false}},"upstreams":[{"dial":"app.lan:8080"}]}`))
	require.NoError(t, ApplyUpstreamResolver(doc, testResolver, nil))
	transport := doc.(map[string]any)["transport"].(map[string]any)
	assert.Equal(t, map[string]any{"insecure_skip_verify": false}, transport["tls"])
	assert.Equal(t, "http", transport["protocol"])
}

func TestApplyUpstreamResolver_ReachesSubroutes(t *testing.T) {
	t.Parallel()
	doc := decodeNumbers(t, []byte(`{"handler":"subroute","routes":[{"handle":[`+
		`{"handler":"reverse_proxy","transport":{"protocol":"http","tls":{}},"upstreams":[{"dial":"app.lan:8080"}]}`+
		`]}]}`))
	require.NoError(t, ApplyUpstreamResolver(doc, testResolver, nil))
	handlers := reverseProxyObjects(t, doc)
	require.Len(t, handlers, 1)
	assert.Equal(t, []any{"udp/127.0.0.1:41000"}, resolverAddresses(t, handlers[0]))
}
