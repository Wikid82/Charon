package caddy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImporter_ExtractHosts_TLSConnectionPolicyAndDialWithoutPort(t *testing.T) {
	// Build a sample Caddy JSON with TLSConnectionPolicies and reverse_proxy with dial host:port and host-only dials
	cfg := CaddyConfig{
		Apps: &CaddyApps{
			HTTP: &CaddyHTTP{
				Servers: map[string]*CaddyServer{
					"srv": {
						Listen: []string{":443"},
						Routes: []*CaddyRoute{
							{
								Match: []*CaddyMatcher{{Host: []string{"example.com"}}},
								Handle: []*CaddyHandler{
									{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "app:9000"}}},
								},
							},
							{
								Match: []*CaddyMatcher{{Host: []string{"nport.example.com"}}},
								Handle: []*CaddyHandler{
									{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "app"}}},
								},
							},
						},
						TLSConnectionPolicies: struct{}{},
					},
				},
			},
		},
	}
	out, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(out)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 2)
	// First host should have scheme https because Listen :443
	require.Equal(t, "https", res.Hosts[0].ForwardScheme)
	// second host with dial 'app' should be parsed with default port 80
	require.Equal(t, 80, res.Hosts[1].ForwardPort)
}

func TestExtractHandlers_Subroute_WithUnsupportedSubhandle(t *testing.T) {
	// Build a handler with subroute whose handle contains a non-map item
	h := []*CaddyHandler{
		{Handler: "subroute", Routes: []interface{}{map[string]interface{}{"handle": []interface{}{"not-a-map", map[string]interface{}{"handler": "reverse_proxy"}}}}},
	}
	importer := NewImporter("")
	res := importer.extractHandlers(h)
	// Should ignore the non-map and keep the reverse_proxy handler
	require.Len(t, res, 1)
	require.Equal(t, "reverse_proxy", res[0].Handler)
}

func TestExtractHandlers_Subroute_WithNonMapRoutes(t *testing.T) {
	h := []*CaddyHandler{
		{Handler: "subroute", Routes: []interface{}{"not-a-map"}},
	}
	importer := NewImporter("")
	res := importer.extractHandlers(h)
	require.Len(t, res, 0)
}

func TestImporter_ExtractHosts_UpstreamsNonMapAndWarnings(t *testing.T) {
	cfg := CaddyConfig{
		Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
			Listen: []string{":80"},
			Routes: []*CaddyRoute{{
				Match:  []*CaddyMatcher{{Host: []string{"warn.example.com"}}},
				Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{"nonnmap"}}, {Handler: "rewrite"}, {Handler: "file_server"}},
			}},
		}}}},
	}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	require.Contains(t, res.Hosts[0].Warnings[0], "Rewrite rules not supported")
	require.Contains(t, res.Hosts[0].Warnings[1], "File server directives not supported")
}

func TestBackupCaddyfile_ReadFailure(t *testing.T) {
	tmp := t.TempDir()
	// original file does not exist
	_, err := BackupCaddyfile("/does/not/exist", tmp)
	require.Error(t, err)
}

func TestExtractHandlers_Subroute_EmptyAndHandleNotArray(t *testing.T) {
	// Empty routes array
	h := []*CaddyHandler{
		{Handler: "subroute", Routes: []interface{}{}},
	}
	importer := NewImporter("")
	res := importer.extractHandlers(h)
	require.Len(t, res, 0)

	// Routes with a map but handle is not an array
	h2 := []*CaddyHandler{
		{Handler: "subroute", Routes: []interface{}{map[string]interface{}{"handle": "not-an-array"}}},
	}
	res2 := importer.extractHandlers(h2)
	require.Len(t, res2, 0)
}

func TestImporter_ExtractHosts_ReverseProxyNoUpstreams(t *testing.T) {
	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"noups.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	// No upstreams should leave ForwardHost empty and ForwardPort 0
	require.Equal(t, "", res.Hosts[0].ForwardHost)
	require.Equal(t, 0, res.Hosts[0].ForwardPort)
}

func TestBackupCaddyfile_Success(t *testing.T) {
	tmp := t.TempDir()
	originalFile := filepath.Join(tmp, "Caddyfile")
	data := []byte("original-data")
	os.WriteFile(originalFile, data, 0644)
	backupDir := filepath.Join(tmp, "backup")
	path, err := BackupCaddyfile(originalFile, backupDir)
	require.NoError(t, err)
	// Backup file should exist and contain same data
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, b)
}

func TestExtractHandlers_Subroute_WithHeadersUpstreams(t *testing.T) {
	h := []*CaddyHandler{
		{Handler: "subroute", Routes: []interface{}{map[string]interface{}{"handle": []interface{}{map[string]interface{}{"handler": "reverse_proxy", "upstreams": []interface{}{map[string]interface{}{"dial": "app:8080"}}, "headers": map[string]interface{}{"Upgrade": []interface{}{"websocket"}}}}}}},
	}
	importer := NewImporter("")
	res := importer.extractHandlers(h)
	require.Len(t, res, 1)
	require.Equal(t, "reverse_proxy", res[0].Handler)
	// Upstreams should be present in extracted handler
	_, ok := res[0].Upstreams.([]interface{})
	require.True(t, ok)
	_, ok = res[0].Headers.(map[string]interface{})
	require.True(t, ok)
}

func TestImporter_ExtractHosts_DuplicateHost(t *testing.T) {
	cfg := CaddyConfig{
		Apps: &CaddyApps{
			HTTP: &CaddyHTTP{
				Servers: map[string]*CaddyServer{
					"srv": {
						Listen: []string{":80"},
						Routes: []*CaddyRoute{{
							Match:  []*CaddyMatcher{{Host: []string{"dup.example.com"}}},
							Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "one:80"}}}},
						}},
					},
					"srv2": {
						Listen: []string{":80"},
						Routes: []*CaddyRoute{{
							Match:  []*CaddyMatcher{{Host: []string{"dup.example.com"}}},
							Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "two:80"}}}},
						}},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	// Duplicate should be captured in Conflicts
	require.Len(t, res.Conflicts, 1)
	require.Equal(t, "dup.example.com", res.Conflicts[0])
}

func TestBackupCaddyfile_WriteFailure(t *testing.T) {
	tmp := t.TempDir()
	originalFile := filepath.Join(tmp, "Caddyfile")
	os.WriteFile(originalFile, []byte("original"), 0644)
	// Create backup dir and make it readonly to prevent writing (best-effort)
	backupDir := filepath.Join(tmp, "backup")
	os.MkdirAll(backupDir, 0555)
	_, err := BackupCaddyfile(originalFile, backupDir)
	// Might error due to write permission; accept both success or failure depending on platform
	if err != nil {
		require.Error(t, err)
	} else {
		entries, _ := os.ReadDir(backupDir)
		require.True(t, len(entries) > 0)
	}
}

func TestImporter_ExtractHosts_SSLForcedByDomainScheme(t *testing.T) {
	// Domain contains scheme prefix, which should set SSLForced
	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"https://secure.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "one:80"}}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	require.Equal(t, true, res.Hosts[0].SSLForced)
	require.Equal(t, "https", res.Hosts[0].ForwardScheme)
}

func TestImporter_ExtractHosts_MultipleHostsInMatch(t *testing.T) {
	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"m1.example.com", "m2.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "one:80"}}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 2)
}

func TestImporter_ExtractHosts_UpgradeHeaderAsString(t *testing.T) {
	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"ws.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "one:80"}}, Headers: map[string]interface{}{"Upgrade": []string{"websocket"}}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	// Websocket support should be detected after JSON roundtrip
	require.True(t, res.Hosts[0].WebsocketSupport)
}

func TestImporter_ExtractHosts_SscanfFailureOnPort(t *testing.T) {
	// Trigger net.SplitHostPort success but Sscanf failing
	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"sscanf.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "127.0.0.1:eighty"}}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	// Sscanf should fail and default to port 80
	require.Equal(t, 80, res.Hosts[0].ForwardPort)
}

func TestImporter_ExtractHosts_PartsSscanfFail(t *testing.T) {
	// Trigger net.SplitHostPort fail but strings.Split parts with non-numeric port
	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"parts.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "tcp/127.0.0.1:badport"}}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	require.Equal(t, 80, res.Hosts[0].ForwardPort)
}

func TestImporter_ExtractHosts_PartsEmptyPortField(t *testing.T) {
	// net.SplitHostPort fails (missing port) but strings.Split returns two parts with empty port
	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"emptyparts.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "tcp/127.0.0.1:"}}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	require.Equal(t, 80, res.Hosts[0].ForwardPort)
}

func TestImporter_ExtractHosts_ForceSplitFallback_PartsNumericPort(t *testing.T) {
	// Force the fallback split behavior to hit len(parts)==2 branch
	orig := forceSplitFallback
	forceSplitFallback = true
	defer func() { forceSplitFallback = orig }()

	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"forced.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "127.0.0.1:8181"}}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	require.Equal(t, "127.0.0.1", res.Hosts[0].ForwardHost)
	require.Equal(t, 8181, res.Hosts[0].ForwardPort)
}

func TestImporter_ExtractHosts_ForceSplitFallback_PartsSscanfFail(t *testing.T) {
	// Force the fallback split behavior with non-numeric port to hit Sscanf error branch
	orig := forceSplitFallback
	forceSplitFallback = true
	defer func() { forceSplitFallback = orig }()

	cfg := CaddyConfig{Apps: &CaddyApps{HTTP: &CaddyHTTP{Servers: map[string]*CaddyServer{"srv": {
		Listen: []string{":80"},
		Routes: []*CaddyRoute{{
			Match:  []*CaddyMatcher{{Host: []string{"forcedfail.example.com"}}},
			Handle: []*CaddyHandler{{Handler: "reverse_proxy", Upstreams: []interface{}{map[string]interface{}{"dial": "127.0.0.1:notnum"}}}},
		}},
	}}}}}
	b, _ := json.Marshal(cfg)
	importer := NewImporter("")
	res, err := importer.ExtractHosts(b)
	require.NoError(t, err)
	require.Len(t, res.Hosts, 1)
	require.Equal(t, 80, res.Hosts[0].ForwardPort)
}

func TestBackupCaddyfile_WriteErrorDeterministic(t *testing.T) {
	tmp := t.TempDir()
	originalFile := filepath.Join(tmp, "Caddyfile")
	os.WriteFile(originalFile, []byte("original-data"), 0644)
	backupDir := filepath.Join(tmp, "backup")
	os.MkdirAll(backupDir, 0755)
	// Determine backup path name the function will use
	pid := fmt.Sprintf("%d", os.Getpid())
	// Pre-create a directory at the exact backup path to ensure write fails with EISDIR
	path := filepath.Join(backupDir, fmt.Sprintf("Caddyfile.%s.backup", pid))
	os.Mkdir(path, 0755)
	_, err := BackupCaddyfile(originalFile, backupDir)
	require.Error(t, err)
}

func TestParseCaddyfile_InvalidPath(t *testing.T) {
	importer := NewImporter("")
	_, err := importer.ParseCaddyfile("")
	require.Error(t, err)

	_, err = importer.ParseCaddyfile(".")
	require.Error(t, err)

	// Path traversal should be rejected
	traversal := ".." + string(os.PathSeparator) + "Caddyfile"
	_, err = importer.ParseCaddyfile(traversal)
	require.Error(t, err)
}

func TestBackupCaddyfile_InvalidOriginalPath(t *testing.T) {
	tmp := t.TempDir()
	// Empty path
	_, err := BackupCaddyfile("", tmp)
	require.Error(t, err)

	// Path traversal rejection
	_, err = BackupCaddyfile(".."+string(os.PathSeparator)+"Caddyfile", tmp)
	require.Error(t, err)
}
