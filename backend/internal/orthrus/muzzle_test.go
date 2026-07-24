package orthrus

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/Wikid82/charon/backend/internal/models"
)

func passthroughHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestMuzzle_AllowlistedGET_Passthrough(t *testing.T) {
	allowed := []string{
		"/_ping",
		"/containers/json",
		"/images/json",
		"/info",
		"/version",
		"/events",
		"/volumes",
		"/networks",
		"/system/df",
	}

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	for _, path := range allowed {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestMuzzle_VersionPrefixStripped_Passthrough(t *testing.T) {
	paths := []string{
		"/v1.44/containers/json",
		"/v1.40/images/json",
		"/v1.41/info",
		"/v1.42/version",
		"/v1.47/_ping",
		"/v1.44/events",
		"/v1.44/volumes",
		"/v1.44/networks",
		"/v1.47/system/df",
	}

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// TestNormalizeDockerPath exercises normalizeDockerPath directly (not only
// through ServeHTTP/the shared corpus) for fast local iteration on the two
// GH #1160 divergence rows that motivated extracting this helper: a
// traversal-disguised version prefix, and a non-numeric fake version prefix.
func TestNormalizeDockerPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "legitimate version prefix stripped",
			in:   "/v1.44/containers/json",
			want: "/containers/json",
		},
		{
			// Divergence row 1 (GH #1160's own example): the version prefix
			// is only revealed after traversal resolution. versionPrefixRe
			// must NOT match here, since it runs against the raw path before
			// path.Clean has resolved "/foo/..".
			name: "traversal-disguised version prefix is not treated as a version prefix",
			in:   "/foo/../v1.44/images/x/json",
			want: "/v1.44/images/x/json",
		},
		{
			// Divergence row 2: non-numeric "version" segment must not match
			// the numeric-anchored regex.
			name: "non-numeric fake version prefix is left intact",
			in:   "/vFOO/containers/json",
			want: "/vFOO/containers/json",
		},
		{
			name: "traversal escaping above root clamps to root",
			in:   "/containers/../../etc/passwd",
			want: "/etc/passwd",
		},
		{
			name: "double slash after legitimate version prefix collapses",
			in:   "/v1.44//containers/json",
			want: "/containers/json",
		},
		{
			name: "version-prefixed traversal resolving to an allowed path",
			in:   "/v1.47/../containers/json",
			want: "/containers/json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeDockerPath(tc.in))
		})
	}
}

func TestMuzzle_POST_Blocked(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	paths := []string{
		"/containers/create",
		"/containers/json",
		"/images/create",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestMuzzle_DELETE_Blocked(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)
	req := httptest.NewRequest(http.MethodDelete, "/containers/abc123", http.NoBody)
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMuzzle_HEAD_Ping_Passthrough(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	for _, path := range []string{"/_ping", "/v1.44/_ping"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodHead, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestMuzzle_HEAD_NonPing_Blocked(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)
	req := httptest.NewRequest(http.MethodHead, "/containers/json", http.NoBody)
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMuzzle_DynamicPaths_Passthrough(t *testing.T) {
	paths := []string{
		"/containers/abc123/json",
		"/v1.44/containers/abc123/json",
		"/containers/abc123/logs",
		"/v1.47/containers/abc123/logs",
		"/containers/abc123/stats",
		"/v1.47/containers/abc123/stats",
		"/containers/abc123/top",
		"/v1.47/containers/abc123/top",
		"/volumes/myvolume",
		"/v1.44/volumes/myvolume",
		"/networks/mynet",
		"/v1.44/networks/mynet",
		"/images/alpine/json",
		"/v1.44/images/alpine/json",
		"/images/nginx:latest/json",
		"/distribution/alpine/json",
		"/v1.44/distribution/alpine/json",
	}

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestMuzzle_UnknownPath_Blocked(t *testing.T) {
	paths := []string{
		"/containers/create",
		"/exec/abc/start",
		"/containers/abc/kill",
		// Non-"/json"-suffixed image endpoints must stay blocked: the
		// prefix/suffix match for /images/* is intentionally narrow.
		"/images/create",
		"/images/ghcr.io/org/repo/history",
		"/images/ghcr.io/org/repo/get",
		"/images/ghcr.io/org/repo/changes",
		"/distribution/create",
	}

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

// TestMuzzle_NamespacedImagePaths_Passthrough proves the fix for the
// namespaced-image-reference bug: /images/*/json and /distribution/*/json
// must match resource identifiers containing multiple "/"-separated
// segments (the overwhelming majority of real-world image references),
// not just single-segment names like "nginx". Uses prefix/suffix matching
// instead of path.Match, whose "*" does not cross "/".
func TestMuzzle_NamespacedImagePaths_Passthrough(t *testing.T) {
	refs := []string{
		"ghcr.io/org/repo",
		"lscr.io/linuxserver/prowlarr",
		"someuser/reponame",
		"registry.example.com/team/project/image",
	}

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	for _, prefix := range []string{"/images/", "/distribution/"} {
		for _, ref := range refs {
			p := prefix + ref + "/json"
			t.Run(p, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, p, http.NoBody)
				rr := httptest.NewRecorder()
				m.ServeHTTP(rr, req)
				assert.Equal(t, http.StatusOK, rr.Code)
			})

			vp := "/v1.44" + prefix + ref + "/json"
			t.Run(vp, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, vp, http.NoBody)
				rr := httptest.NewRecorder()
				m.ServeHTTP(rr, req)
				assert.Equal(t, http.StatusOK, rr.Code)
			})
		}
	}
}

// TestMuzzle_NamespacedImagePaths_NonGET_Blocked confirms the GET-only
// enforcement (which runs unconditionally before any path match in
// ServeHTTP) still rejects writes against namespaced image paths now that
// they pass the allowlist's path check.
func TestMuzzle_NamespacedImagePaths_NonGET_Blocked(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	paths := []string{
		"/images/ghcr.io/org/repo/json",
		"/distribution/ghcr.io/org/repo/json",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

// TestMuzzle_ImageAndDistributionEndpoints_POSTBlocked confirms the two new
// read-only allowlist entries added for update-checker tools do not open a
// write path: POST to either is still rejected, even though method-checking
// already happens unconditionally before any path match in ServeHTTP.
func TestMuzzle_ImageAndDistributionEndpoints_POSTBlocked(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	paths := []string{
		"/images/alpine/json",
		"/distribution/alpine/json",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

// --- Write-mode tests (Section 3.3.3/3.3.4 of the Orthrus write-mode spec) ---

func TestMuzzle_WriteEndpoints_BlockedWhenWriteDisabled(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "", nil)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/containers/create"},
		{http.MethodPost, "/images/create"},
		{http.MethodPost, "/containers/abc123/start"},
		{http.MethodPost, "/containers/abc123/stop"},
		{http.MethodPost, "/containers/abc123/restart"},
		{http.MethodPost, "/containers/abc123/rename"},
		{http.MethodDelete, "/containers/abc123"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestMuzzle_WriteEndpoints_AllowedWhenWriteEnabled(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/images/create"},
		{http.MethodPost, "/containers/abc123/start"},
		{http.MethodPost, "/containers/abc123/stop"},
		{http.MethodPost, "/containers/abc123/restart"},
		{http.MethodPost, "/containers/abc123/rename"},
		{http.MethodDelete, "/containers/abc123"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// TestMuzzle_ContainerRename_QueryParamNameAllowed proves the Dockhand
// image-update rename step end-to-end: Docker's rename endpoint takes the
// new container name via a "?name=" query parameter, not a request body
// (unlike /containers/create), and a real Docker-generated container ID is
// a single path segment (no slashes) — exactly what
// allowedWritePatterns's "/containers/*/rename" path.Match pattern assumes.
// This is Dockhand's standard update pattern: rename the old container out
// of the way (e.g. to "<name>_old") before bringing up the new one under
// the original name — the flow that was failing with a 403 before this fix
// (rename was missing from the write allowlist).
func TestMuzzle_ContainerRename_QueryParamNameAllowed(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)

	// A real Docker-generated container ID: 64 hex characters, single path
	// segment.
	containerID := "3f4d9e2a1b6c8f0d7e5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e"
	req := httptest.NewRequest(http.MethodPost, "/containers/"+containerID+"/rename?name=myapp_old", http.NoBody)
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMuzzle_WriteMode_NonWriteEndpointsStillBlocked(t *testing.T) {
	// Even with write mode on, endpoints outside the fixed seven-operation
	// list (create, images/create, start, stop, restart, rename, delete)
	// — e.g. exec, image delete, build, prune, auth, commit, Swarm/service —
	// must remain permanently blocked. Section 7, Explicit Out-of-Scope.
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/containers/abc123/exec"},
		{http.MethodPost, "/exec/xyz/start"},
		{http.MethodDelete, "/images/nginx"},
		{http.MethodPost, "/build"},
		{http.MethodPost, "/system/prune"},
		{http.MethodPost, "/auth"},
		{http.MethodPost, "/commit"},
		{http.MethodPost, "/networks/create"},
		{http.MethodDelete, "/networks/mynet"},
		{http.MethodPost, "/volumes/create"},
		{http.MethodDelete, "/volumes/myvol"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestMuzzle_ContainersCreate_SafeBodyAllowed(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)

	body := `{"Image":"nginx:latest","HostConfig":{"PortBindings":{"80/tcp":[{"HostPort":"8080"}]},"RestartPolicy":{"Name":"unless-stopped"},"NetworkMode":"my-bridge"}}`
	req := httptest.NewRequest(http.MethodPost, "/containers/create", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestMuzzle_ContainersCreate_SafeBodiesAllowed_CapDropAndUsernsMode covers
// two HostConfig fields a Dockhand-style recreate legitimately sends that a
// blanket exhaustive allowlist previously rejected outright: CapDrop (which
// only ever reduces privilege, unlike CapAdd) and a non-"host" UsernsMode
// value (the daemon-userns-remap-inherited case, not the escape-relevant
// one).
func TestMuzzle_ContainersCreate_SafeBodiesAllowed_CapDropAndUsernsMode(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)

	cases := []struct {
		name string
		body string
	}{
		{"CapDrop", `{"Image":"nginx","HostConfig":{"CapDrop":["ALL"]}}`},
		{"UsernsMode empty (inherit daemon default)", `{"Image":"nginx","HostConfig":{"UsernsMode":""}}`},
		{"UsernsMode named remap", `{"Image":"nginx","HostConfig":{"UsernsMode":"default"}}`},
		{"ContainerIDFile empty", `{"Image":"nginx","HostConfig":{"ContainerIDFile":""}}`},
		{
			"CPU/memory/IO/pid resource-limit family",
			`{"Image":"nginx","HostConfig":{"CpuPeriod":100000,"CpuQuota":50000,"CpuRealtimePeriod":1000000,"CpuRealtimeRuntime":950000,"CpusetCpus":"0-1","CpusetMems":"0","BlkioWeight":500,"KernelMemory":0,"KernelMemoryTCP":0,"MemoryReservation":1048576,"MemorySwappiness":60,"OomKillDisable":false,"OomScoreAdj":0,"PidsLimit":100,"Ulimits":[{"Name":"nofile","Soft":1024,"Hard":2048}],"StorageOpt":{"size":"10G"},"ShmSize":67108864,"CPUCount":0,"CPUPercent":0,"IOMaximumIOps":0,"IOMaximumBandwidth":0}}`,
		},
		{"CgroupnsMode private (Docker default since 20.10)", `{"Image":"nginx","HostConfig":{"CgroupnsMode":"private"}}`},
		{"CgroupnsMode empty (inherit daemon default)", `{"Image":"nginx","HostConfig":{"CgroupnsMode":""}}`},
		{"Tmpfs (container-internal only)", `{"Image":"nginx","HostConfig":{"Tmpfs":{"/tmp":"rw,noexec,nosuid,size=100m"}}}`},
		{"MaskedPaths/ReadonlyPaths (restriction-only)", `{"Image":"nginx","HostConfig":{"MaskedPaths":["/proc/kcore"],"ReadonlyPaths":["/proc/sys"]}}`},
		{"ConsoleSize/Annotations (cosmetic)", `{"Image":"nginx","HostConfig":{"ConsoleSize":[24,80],"Annotations":{"note":"x"}}}`},
		{"Links/PublishAllPorts/DnsOptions (networking convenience)", `{"Image":"nginx","HostConfig":{"Links":["db:db"],"PublishAllPorts":false,"DnsOptions":["timeout:1"]}}`},
		{"Isolation default (Windows-only platform selector)", `{"Image":"nginx","HostConfig":{"Isolation":"default"}}`},
		{"Isolation hyperv", `{"Image":"nginx","HostConfig":{"Isolation":"hyperv"}}`},
		{"Isolation process", `{"Image":"nginx","HostConfig":{"Isolation":"process"}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)
			req := httptest.NewRequest(http.MethodPost, "/containers/create", bytes.NewReader([]byte(tc.body)))
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// TestMuzzle_ContainersCreate_VolumesFrom_AllowedWhenSourceConfigured proves
// the per-agent VolumesFrom source allowlist: a name present in
// Muzzle.allowedVolumesFromSources is accepted, one that isn't is rejected,
// and the mode suffix ("public-config:ro") is stripped before comparison.
func TestMuzzle_ContainersCreate_VolumesFrom_AllowedWhenSourceConfigured(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)

	cases := []struct {
		name string
		body string
	}{
		{"exact name match", `{"Image":"nginx","HostConfig":{"VolumesFrom":["public-config"]}}`},
		{"name with :ro mode suffix stripped before comparison", `{"Image":"nginx","HostConfig":{"VolumesFrom":["public-config:ro"]}}`},
		{"multiple sources, all configured", `{"Image":"nginx","HostConfig":{"VolumesFrom":["public-config","shared-media"]}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", []string{"public-config", "shared-media"})
			req := httptest.NewRequest(http.MethodPost, "/containers/create", bytes.NewReader([]byte(tc.body)))
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestMuzzle_ContainersCreate_DangerousBodiesRejected(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)

	cases := []struct {
		name string
		body string
	}{
		{"Privileged", `{"Image":"nginx","HostConfig":{"Privileged":true}}`},
		{"CapAdd", `{"Image":"nginx","HostConfig":{"CapAdd":["SYS_ADMIN"]}}`},
		{"legacy Binds", `{"Image":"nginx","HostConfig":{"Binds":["/:/host"]}}`},
		{"NetworkMode host", `{"Image":"nginx","HostConfig":{"NetworkMode":"host"}}`},
		{"NetworkMode container:*", `{"Image":"nginx","HostConfig":{"NetworkMode":"container:abc"}}`},
		{"UsernsMode host", `{"Image":"nginx","HostConfig":{"UsernsMode":"host"}}`},
		{"ContainerIDFile non-empty (arbitrary host file creation)", `{"Image":"nginx","HostConfig":{"ContainerIDFile":"/etc/cron.d/pwn"}}`},
		{"CgroupnsMode host", `{"Image":"nginx","HostConfig":{"CgroupnsMode":"host"}}`},
		{"Runtime", `{"Image":"nginx","HostConfig":{"Runtime":"custom-runtime"}}`},
		{"VolumeDriver", `{"Image":"nginx","HostConfig":{"VolumeDriver":"custom-driver"}}`},
		{"VolumesFrom, no source allowlist configured", `{"Image":"nginx","HostConfig":{"VolumesFrom":["other-container"]}}`},
		{"DeviceRequests", `{"Image":"nginx","HostConfig":{"DeviceRequests":[{"Driver":"nvidia","Count":-1}]}}`},
		{"bind-type Mounts", `{"Image":"nginx","HostConfig":{"Mounts":[{"Type":"bind","Source":"/etc","Target":"/x"}]}}`},
		{
			"local-driver bind-mount-via-volume bypass (VolumeOptions.DriverConfig)",
			`{"Image":"nginx","HostConfig":{"Mounts":[{"Type":"volume","Source":"fake","Target":"/etc","VolumeOptions":{"DriverConfig":{"Name":"local","Options":{"type":"none","device":"/etc","o":"bind"}}}}]}}`,
		},
		{"non-empty Devices", `{"Image":"nginx","HostConfig":{"Devices":[{"PathOnHost":"/dev/sda","PathInContainer":"/dev/sda"}]}}`},
		{"non-empty Sysctls", `{"Image":"nginx","HostConfig":{"Sysctls":{"net.ipv4.ip_forward":"1"}}}`},
		{"unrecognized future HostConfig key", `{"Image":"nginx","HostConfig":{"SomeFutureField":true}}`},
		{"malformed JSON", `{"Image": not valid json`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)
			req := httptest.NewRequest(http.MethodPost, "/containers/create", bytes.NewReader([]byte(tc.body)))
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

// TestMuzzle_ContainersCreate_VolumesFrom_RejectedWhenSourceNotConfigured
// proves that having *some* allowlist configured doesn't loosen the check
// into "any VolumesFrom is fine" — only the specific configured names are
// accepted, and a source outside that list is still rejected exactly like
// the unconfigured case.
func TestMuzzle_ContainersCreate_VolumesFrom_RejectedWhenSourceNotConfigured(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", []string{"public-config"})

	body := `{"Image":"nginx","HostConfig":{"VolumesFrom":["not-on-the-allowlist"]}}`
	req := httptest.NewRequest(http.MethodPost, "/containers/create", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMuzzle_ContainersCreate_BodyTooLarge(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)

	oversized := bytes.Repeat([]byte("a"), maxContainerCreateBodyBytes+1)
	body := `{"Image":"nginx","Labels":{"padding":"` + string(oversized) + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/containers/create", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMuzzle_WriteEndpoints_RateLimited(t *testing.T) {
	// Burst of 1, refill effectively never within the test's lifetime — the
	// first write request consumes the only token; the second must 429.
	writeLimiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)

	req1 := httptest.NewRequest(http.MethodPost, "/containers/abc/start", http.NoBody)
	rr1 := httptest.NewRecorder()
	m.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/containers/abc/start", http.NoBody)
	rr2 := httptest.NewRecorder()
	m.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
}

// mockAuditLogger records every SecurityAudit entry passed to LogAudit, for
// assertion in tests. Safe for concurrent use.
type mockAuditLogger struct {
	mu      sync.Mutex
	entries []*models.SecurityAudit
}

func (m *mockAuditLogger) LogAudit(a *models.SecurityAudit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, a)
	return nil
}

func (m *mockAuditLogger) all() []*models.SecurityAudit {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*models.SecurityAudit, len(m.entries))
	copy(out, m.entries)
	return out
}

func TestMuzzle_AuditLog_AllowedWriteRecorded(t *testing.T) {
	logger := &mockAuditLogger{}
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, logger, "agent-uuid-1", nil)

	req := httptest.NewRequest(http.MethodPost, "/containers/abc/start", http.NoBody)
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)

	entries := logger.all()
	require.Len(t, entries, 1)
	assert.Equal(t, "orthrus_write_allowed", entries[0].Action)
	assert.Equal(t, "orthrus_write", entries[0].EventCategory)
	assert.Equal(t, "agent-uuid-1", entries[0].ResourceUUID)
	assert.Contains(t, entries[0].Actor, "agent-uuid-1")
}

func TestMuzzle_AuditLog_BlockedWriteRecorded(t *testing.T) {
	logger := &mockAuditLogger{}
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, logger, "agent-uuid-2", nil)

	body := `{"Image":"nginx","HostConfig":{"Privileged":true}}`
	req := httptest.NewRequest(http.MethodPost, "/containers/create", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)

	entries := logger.all()
	require.Len(t, entries, 1)
	assert.Equal(t, "orthrus_write_blocked", entries[0].Action)
	assert.Contains(t, entries[0].Details, "Privileged")
}

func TestMuzzle_AuditLog_RateLimitedRecorded(t *testing.T) {
	logger := &mockAuditLogger{}
	writeLimiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, logger, "agent-uuid-3", nil)

	req1 := httptest.NewRequest(http.MethodPost, "/containers/abc/start", http.NoBody)
	m.ServeHTTP(httptest.NewRecorder(), req1)
	req2 := httptest.NewRequest(http.MethodPost, "/containers/abc/start", http.NoBody)
	m.ServeHTTP(httptest.NewRecorder(), req2)

	entries := logger.all()
	require.Len(t, entries, 2)
	assert.Equal(t, "orthrus_write_allowed", entries[0].Action)
	assert.Equal(t, "orthrus_write_rate_limited", entries[1].Action)
}

func TestMuzzle_AuditLog_NilLoggerIsNoop(t *testing.T) {
	writeLimiter := rate.NewLimiter(rate.Inf, 100)
	m := NewMuzzle(passthroughHandler(), true, writeLimiter, nil, "agent-uuid", nil)

	req := httptest.NewRequest(http.MethodPost, "/containers/abc/start", http.NoBody)
	rr := httptest.NewRecorder()
	assert.NotPanics(t, func() { m.ServeHTTP(rr, req) })
	assert.Equal(t, http.StatusOK, rr.Code)
}

// --- Shared anti-drift corpus (Section 3.3.5 of the Orthrus write-mode spec) ---
//
// muzzle_corpus.json is the single source of truth this test and
// agent/muzzle/muzzle_test.go's TestFilter_SharedCorpus both load, so the
// backend and agent allowlist implementations are proven to agree on every
// case in it — the concrete mitigation for the drift class that let the
// image/distribution read-only fix land in this package without the
// agent-side copy (fixed later, commits 98a68b67 / eabf358d) once already.

type muzzleCorpusCase struct {
	Description               string          `json:"description"`
	Method                    string          `json:"method"`
	Path                      string          `json:"path"`
	Body                      json.RawMessage `json:"body,omitempty"`
	AgentWriteEnabled         bool            `json:"agent_write_enabled"`
	AllowedVolumesFromSources []string        `json:"allowed_volumes_from_sources,omitempty"`
	WantAllowed               bool            `json:"want_allowed"`
}

func loadMuzzleCorpus(t *testing.T) []muzzleCorpusCase {
	t.Helper()
	data, err := os.ReadFile("testdata/muzzle_corpus.json")
	require.NoError(t, err)
	var cases []muzzleCorpusCase
	require.NoError(t, json.Unmarshal(data, &cases))
	require.NotEmpty(t, cases)
	return cases
}

func TestMuzzle_SharedCorpus(t *testing.T) {
	cases := loadMuzzleCorpus(t)
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			var writeLimiter *rate.Limiter
			if tc.AgentWriteEnabled {
				writeLimiter = rate.NewLimiter(rate.Inf, 1000)
			}
			m := NewMuzzle(passthroughHandler(), tc.AgentWriteEnabled, writeLimiter, nil, "corpus-test-agent", tc.AllowedVolumesFromSources)

			req := httptest.NewRequest(tc.Method, tc.Path, bytes.NewReader(tc.Body))
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)

			allowed := rr.Code == http.StatusOK
			assert.Equal(t, tc.WantAllowed, allowed, "got status %d", rr.Code)
		})
	}
}
