package services

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// oauthStateTTL is how long an issued CSRF state token remains valid
// (spec R3, §3.5): long enough for an admin to complete the provider's
// consent screen, short enough to keep the exposure window small.
const oauthStateTTL = 10 * time.Minute

// oauthStateEntry is what a single issued state token maps to.
type oauthStateEntry struct {
	targetUUID string
	provider   string
	expiresAt  time.Time
}

// OAuthStateStore is an in-memory, single-use, time-bound CSRF state store
// for the Dropbox/Google Drive OAuth2 authorization-code flow (spec §3.4/
// §3.5, Issue #32 Phase 2). Deliberately not persisted to the database — a
// lost in-flight OAuth handshake across a restart is a harmless "please
// click Connect again," and Charon is a single-process binary with no
// multi-replica deployment to coordinate across (spec §3.4 "No new
// tables").
type OAuthStateStore struct {
	mu     sync.Mutex
	states map[string]oauthStateEntry
}

// NewOAuthStateStore constructs an empty OAuthStateStore.
func NewOAuthStateStore() *OAuthStateStore {
	return &OAuthStateStore{states: make(map[string]oauthStateEntry)}
}

// Issue generates a fresh, single-use, oauthStateTTL-bound state token bound
// to targetUUID/provider (spec R3). Also opportunistically sweeps expired
// entries so the map doesn't grow unbounded across a long-running process
// (10-minute TTL keeps this cheap regardless of call volume).
func (s *OAuthStateStore) Issue(targetUUID, provider string) (state string, err error) {
	token, err := generateOAuthState()
	if err != nil {
		return "", fmt.Errorf("oauth state store: generate state: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for k, v := range s.states {
		if now.After(v.expiresAt) {
			delete(s.states, k)
		}
	}

	s.states[token] = oauthStateEntry{
		targetUUID: targetUUID,
		provider:   provider,
		expiresAt:  now.Add(oauthStateTTL),
	}

	return token, nil
}

// Consume looks up state, returning (targetUUID, provider, true) exactly
// once for a state that was actually issued and has not yet expired. The
// entry is deleted on read regardless of outcome (one-time use, spec R3):
// an unknown, already-consumed, or expired state returns ok=false — the
// caller (the OAuth callback route) must treat that identically to any
// other invalid callback, never distinguishing "expired" from "forged" in
// its response (spec §3.8, avoids leaking timing information about valid
// vs. invalid states).
func (s *OAuthStateStore) Consume(state string) (targetUUID, provider string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, found := s.states[state]
	if !found {
		return "", "", false
	}
	delete(s.states, state)

	if time.Now().After(entry.expiresAt) {
		return "", "", false
	}

	return entry.targetUUID, entry.provider, true
}

// generateOAuthState returns a cryptographically random, base64url-encoded
// (unpadded) 32-byte token — 256 bits of entropy, unguessable enough for a
// CSRF state parameter (spec §3.5 "crypto/rand, 32 bytes, base64url").
func generateOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
