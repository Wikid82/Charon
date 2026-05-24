# Dockhand Flapping Fix — Orthrus Reverse Tunnel Keep-Alive Deadlock

**Feature Branch:** `feature/hecate`
**Date:** 2026-05-24
**Status:** Plan complete — ready for implementation

---

## 1. Problem Summary

Dockhand (Docker management UI on VPS, port 3001) reaches HomeLab Docker via Charon's
Orthrus reverse-WebSocket tunnel at `http://charon:3000`. Every request after the first
one fails with `context deadline exceeded` after exactly 30 seconds — the Dockhand poll
interval. This makes the Dockhand container list show as perpetually "flapping": one
successful load, then all subsequent loads time out until the agent reconnects.

---

## 2. Root Cause

### Confirmed failure sequence

```
Dockhand  →  ExternalProxy (0.0.0.0:3000)  →  loopback TCP  →  proxyConn goroutine
          →  yamux stream  →  agent ServeProxy  →  Docker unix socket
```

1. **Request #1**: `http.DefaultTransport` opens a fresh loopback TCP connection → new
   `proxyConn` goroutine → yamux stream #1 → agent `ServeProxy` dials Docker socket →
   response returned. **SUCCESS**.

2. **Keep-alive idle state**: Docker does NOT close the TCP connection after the response
   because `req.Close` is never set in `agent/muzzle/muzzle.go`. The agent's `ServeProxy`
   goroutine blocks inside `io.Copy(w=yamuxStream, conn=dockerSocket)` waiting for more
   data that never arrives (Docker is idle, connection is alive).

3. **Transport pools the connection**: `http.DefaultTransport` considers the loopback TCP
   connection healthy and returns it to its idle pool.

4. **Request #2 (30 s later)**: Transport reuses the pooled loopback TCP connection.
   Bytes of the new request arrive at the `proxyConn` goroutine's loopback TCP socket,
   which forwards them into **yamux stream #1**. The agent is not reading from yamux
   stream #1 — it is stuck in `io.Copy`.

5. **Deadlock**: The yamux receive window on stream #1 fills up. The VPS-side write to
   yamux blocks. The loopback TCP write buffer fills. `httputil.ReverseProxy` write
   blocks. After 30 s, Dockhand's context fires → `context deadline exceeded`.

6. **Self-reinforcing**: Because every Dockhand poll arrives at exactly the 30-second
   interval (same as the timeout), each new request races against Transport's stale-conn
   detection and wins the race, guaranteeing deadlock on every subsequent request.

### Why Fix 1 (agent) is fundamental

Setting `req.Close = true` before `req.Write(conn)` causes `ServeProxy` to send
`Connection: close` to Docker. Docker closes the connection after sending the response.
`io.Copy(w, conn)` receives EOF and returns. The yamux stream closes cleanly. The
`proxyConn` goroutine exits. No goroutine leak. No stale stream.

### Why Fix 2 (server) is belt-and-suspenders (and deployable immediately)

Adding `Transport: &http.Transport{DisableKeepAlives: true}` to the `httputil.ReverseProxy`
in `StartExternalProxy` prevents Transport from ever pooling the loopback TCP connection.
Every request opens a fresh loopback TCP connection → fresh `proxyConn` goroutine → fresh
yamux stream → fresh agent goroutine. The agent can handle it. This fix works with the
**old agent already deployed on HomeLab** and stops the flapping immediately.

---

## 3. Affected Files

| File | Function | Approx. Lines | Change |
|------|----------|---------------|--------|
| `agent/muzzle/muzzle.go` | `ServeProxy` | 111–139 | Add `req.Close = true` before `req.Write(conn)` (~L131) |
| `backend/internal/orthrus/session.go` | `StartExternalProxy` | ~253–330 | Add `Transport: &http.Transport{DisableKeepAlives: true}` to `httputil.ReverseProxy` literal (~L293) |
| `agent/muzzle/muzzle_test.go` | (new tests) | append | Two new tests for connection-close behaviour |
| `backend/internal/orthrus/session_test.go` | (new test) | append | One new test for DisableKeepAlives on external proxy transport |

---

## 4. Exact Code Changes

### 4.1 Fix 1 — `agent/muzzle/muzzle.go` `ServeProxy` (~line 131)

**Before:**
```go
	conn, err := net.Dial("unix", dst)
	if err != nil {
		return fmt.Errorf("muzzle: dial docker socket: %w", err)
	}
	defer conn.Close()

	// Forward the full request (headers + body) to the Docker socket.
	if err := req.Write(conn); err != nil {
		return fmt.Errorf("muzzle: forward request to docker: %w", err)
	}

	// Stream the response back to the caller.
	_, err = io.Copy(w, conn)
	return err
```

**After:**
```go
	conn, err := net.Dial("unix", dst)
	if err != nil {
		return fmt.Errorf("muzzle: dial docker socket: %w", err)
	}
	defer conn.Close()

	// Signal Docker to close the connection after the response so io.Copy
	// below returns on EOF rather than blocking on an idle keep-alive socket.
	// Without this, ServeProxy holds the yamux stream open indefinitely and
	// the server-side Transport reuses the stale loopback connection, causing
	// every subsequent request to deadlock (context deadline exceeded).
	req.Close = true

	// Forward the full request (headers + body) to the Docker socket.
	if err := req.Write(conn); err != nil {
		return fmt.Errorf("muzzle: forward request to docker: %w", err)
	}

	// Stream the response back to the caller.
	_, err = io.Copy(w, conn)
	return err
```

---

### 4.2 Fix 2 — `backend/internal/orthrus/session.go` `StartExternalProxy` (~line 285)

**Before:**
```go
	loopbackTarget := fmt.Sprintf("127.0.0.1:%d", loopbackPort)
	targetURL := &url.URL{Scheme: "http", Host: loopbackTarget}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.Out.Host = ""
		},
		FlushInterval: -1,
	}
```

**After:**
```go
	loopbackTarget := fmt.Sprintf("127.0.0.1:%d", loopbackPort)
	targetURL := &url.URL{Scheme: "http", Host: loopbackTarget}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.Out.Host = ""
		},
		FlushInterval: -1,
		// Disable HTTP keep-alives on the loopback transport so that each
		// Docker API request opens a fresh loopback TCP connection, which
		// maps to a fresh yamux stream that the agent can service without
		// contention. With keep-alives enabled, Transport reuses the loopback
		// connection; the agent goroutine is blocked reading from the stale
		// yamux stream, causing all requests after the first to deadlock.
		Transport: &http.Transport{DisableKeepAlives: true},
	}
```

---

### 4.3 Streaming Endpoints — Safety Confirmation

Both fixes are safe for long-lived streaming responses served by `/events`,
`/containers/*/logs`, and `/containers/*/stats`.

**Fix 1 (`req.Close = true` in `ServeProxy`):**
Docker emits `Connection: close` in the response headers but continues streaming
data until EOF or client disconnect. `io.Copy(w, conn)` in `ServeProxy` terminates
only when the yamux stream write returns an error (i.e., the client/proxy side
disconnects), at which point `defer conn.Close()` fires and tears down the Docker
socket. The streaming data path is unaffected; the connection is not closed
prematurely.

**Fix 2 (`DisableKeepAlives: true` on the loopback transport):**
Each streaming request receives its own dedicated loopback TCP connection and
corresponding yamux stream, which lives for as long as the stream is active. There
is no connection reuse for streaming responses; the connection is only closed when
the response body (stream) finishes or the client disconnects. Correct.

---

## 5. Test Plan

### 5.1 New tests in `agent/muzzle/muzzle_test.go`

#### `TestServeProxy_ConnectionCloseSetOnRequest`

**Purpose:** Verify that `ServeProxy` sets `req.Close = true`, which causes the Docker
socket to be closed after the response, allowing `io.Copy` to return on EOF rather than
blocking forever on an idle keep-alive connection.

**Setup:**
- Start a `net.Listener` on a temp Unix socket path (`t.TempDir()`).
- Serve goroutine: accept one conn, read the full HTTP request using `http.ReadRequest`,
  assert that the `Connection: close` header is present (i.e. `req.Close` was true when
  written), send a minimal `HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello` response,
  then close the socket (honouring the `Connection: close` header).
- Call `f.ServeProxy(socketPath, reqReader, &buf)` in the main goroutine with a 2 s
  deadline enforced by `t.Deadline()` or a `time.AfterFunc` that calls `t.Fatal`.

**Assertions:**
- `ServeProxy` returns `nil` error without hanging.
- `buf.String()` contains `"200 OK"` and `"hello"`.
- The server goroutine observes `req.Header.Get("Connection") == "close"`.

**Signature:**
```go
func TestServeProxy_ConnectionCloseSetOnRequest(t *testing.T) {
```

---

#### `TestServeProxy_CompletesAfterDockerResponse`

**Purpose:** Verify end-to-end that `ServeProxy` returns after receiving a complete HTTP
response from the Docker socket — i.e., it does not hang after the response body is
delivered, because `Connection: close` causes Docker to close the connection.

**Setup:**
- Unix socket server: accept, read request, write complete response (`Content-Length` set
  to exact body length), **close the connection** immediately after writing.
- Call `ServeProxy` with a valid allowed `GET /containers/json` request.

**Assertions:**
- Returns `nil` within a 2 s deadline.
- Response body is forwarded to the writer intact.

**Signature:**
```go
func TestServeProxy_CompletesAfterDockerResponse(t *testing.T) {
```

---

#### `TestServeProxy_StreamingResponseTerminatesOnWriterClose`

**Purpose:** Regression guard for `/events`-style streaming endpoints. Verify that
`ServeProxy` exits promptly when the yamux stream (writer side) is closed, even
when the mock Docker server is still actively writing an infinite response.

**Setup:**
- Unix socket server: accept one connection, read the request, write a
  `Transfer-Encoding: chunked` (or raw byte stream) response, then loop writing
  chunks indefinitely (simulating an infinite `/events` stream).
- Capture the `ResponseWriter` passed to `ServeProxy` in a wrapper that allows the
  test to close it (simulate yamux stream closure) by calling `Close()` on the
  underlying connection or by using a `pipe` whose write-end can be forcibly closed.
- Call `ServeProxy` in a goroutine. After it has started reading (signal via a
  `chan struct{}`), close the writer (yamux stream side).

**Assertions:**
- `ServeProxy` returns within a short deadline (e.g., 2 s) after the writer is
  closed. Any hang indicates the `io.Copy` is not respecting write errors.
- The mock Docker server goroutine eventually receives a write error (confirming
  `defer conn.Close()` in `ServeProxy` fires).

**Signature:**
```go
func TestServeProxy_StreamingResponseTerminatesOnWriterClose(t *testing.T) {
```

---

### 5.2 New test in `backend/internal/orthrus/session_test.go`

#### `TestStartExternalProxy_TransportDisablesKeepAlives`

**Purpose:** Verify that sequential HTTP requests through the external proxy each open a
new connection to the loopback target — confirming that `DisableKeepAlives: true` is in
effect and that Transport does not reuse stale connections.

**Approach (behavioural):**
Since `Transport` is embedded inside `httputil.ReverseProxy` which is constructed inside
`StartExternalProxy`, we test the observable effect: a mock loopback HTTP server counts
how many distinct TCP connections it accepts for two sequential requests. With
`DisableKeepAlives: true` the count must be 2.

**Setup steps:**
1. Start a `httptest.NewServer` that records the `RemoteAddr` of each request. Because
   the loopback target is a real TCP server (not the yamux stack), inject the mock by
   temporarily pointing the external proxy at it.
2. Create a real `AgentSession` using `testWSPair` (existing helper). Start the loopback
   proxy with `StartDockerProxy(0)` to allocate a port, then use `GetProxyAddr()`.
   Override the loopback port by aliasing the mock server's address — achieved by having
   the mock server listen on a random port and setting `proxyPort` via an unexported field
   assignment in the test package (same package `orthrus`).
3. Call `StartExternalProxy(0)` to bind on any free port.
4. Make two sequential `http.Get` requests (using a fresh `http.Client` with no keep-alive
   override — the fix must provide keep-alive isolation at the server, not the test client)
   to `http://localhost:<extPort>/containers/json`.
5. Assert that the mock server received 2 requests on 2 distinct remote addresses
   (connection count == 2).

**Signature:**
```go
func TestStartExternalProxy_TransportDisablesKeepAlives(t *testing.T) {
```

**Fallback:** If bootstrapping a full `AgentSession` with a mock loopback is prohibitively
complex, an acceptable alternative is to use `reflect` to inspect
`rp.Transport.(*http.Transport).DisableKeepAlives` after calling `StartExternalProxy`.
Prefer the behavioural test.

---

## 6. Commit Strategy

One commit, four files:

```
fix(orthrus): prevent keep-alive deadlock on repeated Docker API requests

Every Docker API request after the first timed out with "context deadline
exceeded" because http.DefaultTransport reused the loopback TCP connection
to the proxyConn goroutine. The yamux stream on the agent side was blocked
in io.Copy waiting on an idle Docker socket, so the reused connection had
no reader; the yamux receive window filled, writes blocked, and Dockhand's
30 s context expired.

Fix 1 (agent — muzzle.go): Set req.Close = true before forwarding the
request to the Docker socket. Docker sends Connection: close in the
response, the socket closes on EOF, io.Copy returns, and the yamux stream
is released cleanly.

Fix 2 (server — session.go): Add Transport: &http.Transport{DisableKeepAlives: true}
to the httputil.ReverseProxy in StartExternalProxy. Each request opens a
fresh loopback TCP connection, a fresh proxyConn goroutine, and a fresh
yamux stream, so the agent always has a clean goroutine to handle it.
This fix works with the already-deployed agent and stops the flapping
immediately.

Closes: Dockhand tunnel flapping (every-30s context deadline exceeded)
```

**Files in commit:**
```
agent/muzzle/muzzle.go
agent/muzzle/muzzle_test.go
backend/internal/orthrus/session.go
backend/internal/orthrus/session_test.go
```

---

## 7. Deployment Notes

### Fix 2 first (immediate — no agent rebuild needed)

Fix 2 is server-side only. Deploying a new Charon server image to the VPS stops the
flapping immediately because every request now gets a fresh yamux stream, even with the
old agent code. Deploy this as soon as it passes CI.

**Steps:**
1. Build and push new Charon VPS image.
2. `docker compose up -d charon` on the VPS.
3. Verify in Dockhand that containers load on the first and second poll (30 s apart).

### Fix 1 second (agent rebuild — HomeLab deploy)

Fix 1 requires rebuilding the HomeLab agent image. Without it, each request still opens a
new yamux stream (Fix 2 ensures this), and the previous stream terminates cleanly:
when Fix 2 causes the loopback connection to close, `proxyConn`'s
`io.Copy(stream, conn)` returns, which calls `stream.Close()`. The agent's
`io.Copy(w, dockerSocket)` then fails immediately because the yamux stream is
closed, causing `ServeProxy` to exit promptly. There is no ~75-second linger.
Fix 1 is still valuable because it closes the Docker socket cleanly rather than
relying on the server-side teardown cascade, making goroutine lifecycles explicit.

**Steps:**
1. Build new agent image on HomeLab (or cross-compile and push from CI).
2. `docker compose pull && docker compose up -d charon-agent` on HomeLab.
3. Verify agent reconnects; `GetExternalProxyStatus` shows `active: true`.
4. Confirm no goroutine growth in Charon logs over several minutes.

### Pre-deploy checklist

- [ ] `go test ./agent/muzzle/... ./backend/internal/orthrus/...` passes locally
- [ ] `bash scripts/go-test-coverage.sh` stays above coverage threshold
- [ ] `bash scripts/local-patch-report.sh` shows changed lines covered
- [ ] E2E container health check passes (`docker compose exec charon curl -sf http://localhost:8080/health`)
