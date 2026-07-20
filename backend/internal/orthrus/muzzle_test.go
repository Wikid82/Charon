package orthrus

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

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

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestMuzzle_POST_Blocked(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

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
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")
	req := httptest.NewRequest(http.MethodDelete, "/containers/abc123", http.NoBody)
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMuzzle_HEAD_Ping_Passthrough(t *testing.T) {
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

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
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")
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

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

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

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

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

	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

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
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

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
	m := NewMuzzle(passthroughHandler(), false, nil, nil, "")

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
