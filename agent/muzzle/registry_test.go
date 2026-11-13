package muzzle_test

// Mirrored table: backend/internal/orthrus/muzzle_registry_test.go carries the
// same cases against the backend-side Muzzle. Keep the two tables identical.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/agent/muzzle"
)

var registryLookupCases = []struct {
	ref     string
	allowed bool
}{
	// Allowed.
	{"nginx", true},
	{"library/nginx:1.27", true},
	{"ghcr.io/org/repo:tag", true},
	{"lscr.io/linuxserver/prowlarr", true},
	{"registry.example.com:5000/app@sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", true},
	{"intranet/x", true},

	// Refused: IP literals, localhost, single-label hosts, shorthand IPv4.
	{"127.0.0.1:5000/x", false},
	{"10.0.0.5/x", false},
	{"[::1]:5000/x", false},
	{"localhost/x", false},
	{"LOCALHOST:5000/x", false},
	{"foo.localhost/x", false},
	{"myregistry:5000/x", false},
	{"127.1:5000/x", false},
	{"10.1/x", false},
	{"0x7f.1/x", false},
	{"bad ref!/x", false},
}

func TestFilter_RegistryLookupRule(t *testing.T) {
	f := muzzle.New(false)
	for _, tc := range registryLookupCases {
		t.Run(tc.ref, func(t *testing.T) {
			assert.Equal(t, tc.allowed, f.Allow("GET", "/distribution/"+tc.ref+"/json"))
		})
	}
}

func TestFilter_RegistryLookupRule_VersionedPath(t *testing.T) {
	f := muzzle.New(false)
	assert.False(t, f.Allow("GET", "/v1.44/distribution/127.1:5000/x/json"))
	assert.True(t, f.Allow("GET", "/v1.44/distribution/nginx/json"))
}

func TestFilter_RegistryLookupRule_WriteEnabledPassesThrough(t *testing.T) {
	f := muzzle.New(true)
	assert.True(t, f.Allow("GET", "/distribution/127.0.0.1:5000/x/json"))
	assert.True(t, f.Allow("GET", "/distribution/nginx/json"))
}
