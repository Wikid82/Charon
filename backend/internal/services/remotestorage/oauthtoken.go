package remotestorage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// TokenSaver persists a refreshed OAuth2 token back to a target's encrypted
// secrets blob. Implemented by BackupRemoteService (internal/services) so
// this package stays GORM-free — the same reason remotestorage is its own
// package at all (see the package doc in remotestorage.go). dropbox.go and
// googledrive.go (Commit 3) construct their *http.Client via NewClient
// below, passing a TokenSaver so a transparent mid-flight refresh survives
// process restarts (spec §3.5, Issue #32 Phase 2).
type TokenSaver interface {
	SaveToken(ctx context.Context, accessToken, refreshToken string, expiresAt time.Time) error
}

// RemoteTargetSecrets is the subset of a target's decrypted secrets needed
// by the OAuth-based providers (dropbox.go/googledrive.go, Commit 3). It
// mirrors (but is intentionally a separate, package-local type from)
// services.RemoteTargetSecrets — remotestorage.New's "dropbox"/"google_drive"
// cases adapt the plain map[string]string secrets bag BackupRemoteService
// already decrypts into this shape via secretsFromMap (remotestorage.go).
type RemoteTargetSecrets struct {
	OAuthClientSecret string
	OAuthAccessToken  string
	OAuthRefreshToken string
	OAuthExpiresAt    string // RFC3339; empty means the OAuth flow hasn't completed yet.
}

// parseOAuthExpiry parses raw (expected RFC3339, RemoteTargetSecrets.OAuthExpiresAt's
// format) into a time.Time. An empty or unparsable value returns the zero
// time, which oauth2.Token treats as "already expired" — the safe default,
// forcing an immediate refresh attempt rather than silently reusing a token
// whose real expiry is unknown.
func parseOAuthExpiry(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// ErrOAuthNotConnected is returned when a dropbox/google_drive uploader is
// constructed for a target that has not yet completed its OAuth
// authorization flow (spec §3.5/§3.9, Commit 3).
var ErrOAuthNotConnected = errors.New("oauth authorization required before this target can be used")

// ErrOAuthRevoked is returned when a token refresh attempt fails because the
// stored refresh token was revoked out-of-band by the user at the provider
// (spec R6/§3.9, Commit 3).
var ErrOAuthRevoked = errors.New("oauth refresh token was revoked")

// persistingTokenSource wraps an oauth2.TokenSource so that every time
// Token() returns a *different* access token than the one most recently
// observed (i.e. a transparent refresh happened), the new token is
// persisted via saver before the caller proceeds. This is the standard
// idiom recommended for golang.org/x/oauth2 consumers that need refreshed
// tokens to survive process restarts (spec §3.5). A refresh failure whose
// underlying cause is the provider's RFC 6749 "invalid_grant" response
// (the refresh token itself was revoked) is translated to the sentinel
// ErrOAuthRevoked so callers can distinguish "reconnect required" from an
// ordinary transient network/auth failure.
type persistingTokenSource struct {
	ctx   context.Context
	base  oauth2.TokenSource
	saver TokenSaver

	mu       sync.Mutex
	lastSeen string
}

// Token implements oauth2.TokenSource.
func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
			return nil, ErrOAuthRevoked
		}
		return nil, err
	}

	p.mu.Lock()
	changed := tok.AccessToken != p.lastSeen
	if changed {
		p.lastSeen = tok.AccessToken
	}
	p.mu.Unlock()

	if changed && p.saver != nil {
		if saveErr := p.saver.SaveToken(p.ctx, tok.AccessToken, tok.RefreshToken, tok.Expiry); saveErr != nil {
			return nil, fmt.Errorf("oauthtoken: persist refreshed token: %w", saveErr)
		}
	}

	return tok, nil
}

// NewClient wraps conf's TokenSource (seeded with tok) in the persisting
// layer above and returns an *http.Client that authorizes every request
// with it — spec §3.5. saver may be nil in tests that don't care about
// persistence (a refresh then simply isn't saved).
func NewClient(ctx context.Context, conf oauth2.Config, tok *oauth2.Token, saver TokenSaver) *http.Client {
	var lastSeen string
	if tok != nil {
		lastSeen = tok.AccessToken
	}
	pts := &persistingTokenSource{
		ctx:      ctx,
		base:     conf.TokenSource(ctx, tok),
		saver:    saver,
		lastSeen: lastSeen,
	}
	return oauth2.NewClient(ctx, pts)
}
