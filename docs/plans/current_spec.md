# Implementation Plan: WebSocket X-Forwarded Headers Fix

## Overview

**Issue**: When WebSocket support is enabled on a Proxy Host, Charon correctly adds `Upgrade` and `Connection` header passthrough, but does NOT add `X-Forwarded-*` headers. Many applications (like FileFlows using SignalR) require these headers to properly handle WebSocket connections behind a reverse proxy.

**Solution**: Add `X-Forwarded-Proto`, `X-Forwarded-Host`, and `X-Real-IP` headers when `enableWS` is true.

---

## Files to Modify

### 1. `backend/internal/caddy/types.go`

**Location**: Lines 124-127 (WebSocket support block in `ReverseProxyHandler`)

#### Current Code

```go
	// WebSocket support
	if enableWS {
		setHeaders["Upgrade"] = []string{"{http.request.header.Upgrade}"}
		setHeaders["Connection"] = []string{"{http.request.header.Connection}"}
	}
```

#### New Code

```go
	// WebSocket support
	if enableWS {
		setHeaders["Upgrade"] = []string{"{http.request.header.Upgrade}"}
		setHeaders["Connection"] = []string{"{http.request.header.Connection}"}
		// Add X-Forwarded headers for WebSocket proxy awareness
		// Required by many apps (e.g., SignalR, FileFlows) to properly handle
		// WebSocket connections behind a reverse proxy
		setHeaders["X-Forwarded-Proto"] = []string{"{http.request.scheme}"}
		setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
		setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
	}
```

---

### 2. `backend/internal/caddy/types_extra_test.go`

**Location**: End of file (add new test functions)

#### New Test Functions

```go
func TestReverseProxyHandler_WebSocketHeaders(t *testing.T) {
	// Test: WebSocket enabled should include X-Forwarded headers
	h := ReverseProxyHandler("app:8080", true, "none")
	require.Equal(t, "reverse_proxy", h["handler"])

	hdrs, ok := h["headers"].(map[string]interface{})
	require.True(t, ok, "expected headers map when enableWS=true")

	req, ok := hdrs["request"].(map[string]interface{})
	require.True(t, ok, "expected request headers")

	set, ok := req["set"].(map[string][]string)
	require.True(t, ok, "expected set headers")

	// Verify WebSocket passthrough headers
	require.Contains(t, set, "Upgrade", "Upgrade header should be set for WebSocket")
	require.Equal(t, []string{"{http.request.header.Upgrade}"}, set["Upgrade"])

	require.Contains(t, set, "Connection", "Connection header should be set for WebSocket")
	require.Equal(t, []string{"{http.request.header.Connection}"}, set["Connection"])

	// Verify X-Forwarded headers for proxy awareness
	require.Contains(t, set, "X-Forwarded-Proto", "X-Forwarded-Proto should be set for WebSocket")
	require.Equal(t, []string{"{http.request.scheme}"}, set["X-Forwarded-Proto"])

	require.Contains(t, set, "X-Forwarded-Host", "X-Forwarded-Host should be set for WebSocket")
	require.Equal(t, []string{"{http.request.host}"}, set["X-Forwarded-Host"])

	require.Contains(t, set, "X-Real-IP", "X-Real-IP should be set for WebSocket")
	require.Equal(t, []string{"{http.request.remote.host}"}, set["X-Real-IP"])
}

func TestReverseProxyHandler_NoWebSocketNoForwardedHeaders(t *testing.T) {
	// Test: WebSocket disabled with no application should NOT have X-Forwarded headers
	h := ReverseProxyHandler("app:8080", false, "none")
	require.Equal(t, "reverse_proxy", h["handler"])

	// With enableWS=false and application="none", there should be no headers config
	_, ok := h["headers"]
	require.False(t, ok, "expected no headers when enableWS=false and application=none")
}
```

---

## Implementation Steps

1. **Modify `types.go`**
   - Open [backend/internal/caddy/types.go](backend/internal/caddy/types.go#L124)
   - Locate the WebSocket support block (lines 124-127)
   - Add the three X-Forwarded header lines after the Connection header

2. **Add tests to `types_extra_test.go`**
   - Open [backend/internal/caddy/types_extra_test.go](backend/internal/caddy/types_extra_test.go)
   - Add `TestReverseProxyHandler_WebSocketHeaders` function
   - Add `TestReverseProxyHandler_NoWebSocketNoForwardedHeaders` function

3. **Run tests**
   - Execute: `cd backend && go test ./internal/caddy/... -v -run "TestReverseProxy"`
   - Verify all tests pass

4. **Verify existing tests still pass**
   - Execute: `cd backend && go test ./internal/caddy/... -v`
   - Ensure no regressions

---

## Test Verification Matrix

| Scenario | enableWS | application | Expected Headers |
|----------|----------|-------------|------------------|
| WebSocket only | `true` | `"none"` | Upgrade, Connection, X-Forwarded-Proto, X-Forwarded-Host, X-Real-IP |
| WebSocket + Plex | `true` | `"plex"` | Upgrade, Connection, X-Forwarded-Proto, X-Forwarded-Host, X-Real-IP, X-Plex-* |
| WebSocket + Jellyfin | `true` | `"jellyfin"` | Upgrade, Connection, X-Forwarded-Proto, X-Forwarded-Host, X-Real-IP |
| No WebSocket, no app | `false` | `"none"` | No headers config |
| No WebSocket + Plex | `false` | `"plex"` | X-Plex-*, X-Real-IP, X-Forwarded-Host |

---

## Definition of Done Checklist

- [ ] `types.go` modified to add X-Forwarded headers in WebSocket block
- [ ] `TestReverseProxyHandler_WebSocketHeaders` test added and passing
- [ ] `TestReverseProxyHandler_NoWebSocketNoForwardedHeaders` test added and passing
- [ ] All existing tests in `backend/internal/caddy/` pass
- [ ] `go vet ./...` passes with no warnings
- [ ] Code follows project conventions (comments, naming)

---

## Risk Assessment

**Low Risk**: This change only adds headers when `enableWS=true`. It does not modify existing logic for application-specific headers (Plex, Jellyfin, etc.) which already set some of these headers. The WebSocket block is independent and executes before the application-specific switch statement.

**Note on Header Overlap**: For applications like `"plex"` and `"jellyfin"` that already set `X-Real-IP` and `X-Forwarded-Host`, the WebSocket block will set them first, then the application block will overwrite with the same values. This is harmless as the values are identical.

---

## Related Files Reference

| File | Purpose |
|------|---------|
| [backend/internal/caddy/types.go](backend/internal/caddy/types.go) | Main implementation file |
| [backend/internal/caddy/types_test.go](backend/internal/caddy/types_test.go) | Basic handler tests |
| [backend/internal/caddy/types_extra_test.go](backend/internal/caddy/types_extra_test.go) | Extended handler tests |
