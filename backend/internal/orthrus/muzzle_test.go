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
		"/containers/json",
		"/images/json",
		"/info",
		"/version",
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

func TestMuzzle_UnknownPath_Blocked(t *testing.T) {
	paths := []string{
		"/containers/create",
		"/exec/abc/start",
		"/containers/abc/kill",
		"/networks/create",
		"/_ping",
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
