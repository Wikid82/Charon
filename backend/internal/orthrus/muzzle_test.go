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

	m := NewMuzzle(passthroughHandler())

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

	m := NewMuzzle(passthroughHandler())

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
	m := NewMuzzle(passthroughHandler())

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
	m := NewMuzzle(passthroughHandler())
	req := httptest.NewRequest(http.MethodDelete, "/containers/abc123", http.NoBody)
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMuzzle_HEAD_Ping_Passthrough(t *testing.T) {
	m := NewMuzzle(passthroughHandler())

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
	m := NewMuzzle(passthroughHandler())
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
	}

	m := NewMuzzle(passthroughHandler())

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
	}

	m := NewMuzzle(passthroughHandler())

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}
