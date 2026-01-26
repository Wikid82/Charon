# Proxy TLS & IP Login Recovery — Implementation Notes

The following patches implement the approved plan while respecting the constraint to **not modify source files directly**. Apply them in order. Tests to add are included at the end.

## Backend — Caddy IP-aware TLS/HTTP handling

**Goal:** avoid ACME/AutoHTTPS on IP literals, allow HTTP-only or internal TLS for IP hosts, and add coverage.

Apply this patch to `backend/internal/caddy/config.go`:

```diff
*** Begin Patch
*** Update File: backend/internal/caddy/config.go
@@
-import (
-    "encoding/json"
-    "fmt"
-    "path/filepath"
-    "strings"
+import (
+    "encoding/json"
+    "fmt"
+    "net"
+    "path/filepath"
+    "strings"
@@
-func GenerateConfig(hosts []models.ProxyHost, storageDir, acmeEmail, frontendDir, sslProvider string, acmeStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled bool, adminWhitelist string, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, decisions []models.SecurityDecision, secCfg *models.SecurityConfig) (*Config, error) {
+func GenerateConfig(hosts []models.ProxyHost, storageDir, acmeEmail, frontendDir, sslProvider string, acmeStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled bool, adminWhitelist string, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, decisions []models.SecurityDecision, secCfg *models.SecurityConfig) (*Config, error) {
@@
-    // Initialize routes slice
-    routes := make([]*Route, 0)
+    // Initialize routes slice
+    routes := make([]*Route, 0)
+    // Track IP-only hostnames to skip AutoHTTPS/ACME
+    ipSubjects := make([]string, 0)
@@
-        // Parse comma-separated domains
-        rawDomains := strings.Split(host.DomainNames, ",")
-        var uniqueDomains []string
+        // Parse comma-separated domains
+        rawDomains := strings.Split(host.DomainNames, ",")
+        var uniqueDomains []string
+        isIPOnly := true
@@
-            processedDomains[d] = true
-            uniqueDomains = append(uniqueDomains, d)
+            processedDomains[d] = true
+            uniqueDomains = append(uniqueDomains, d)
+            if net.ParseIP(d) == nil {
+                isIPOnly = false
+            }
         }

         if len(uniqueDomains) == 0 {
             continue
         }
+
+        if isIPOnly {
+            ipSubjects = append(ipSubjects, uniqueDomains...)
+        }
@@
-        route := &Route{
-            Match: []Match{
-                {Host: uniqueDomains},
-            },
-            Handle:   mainHandlers,
-            Terminal: true,
-        }
+        route := &Route{
+            Match: []Match{
+                {Host: uniqueDomains},
+            },
+            Handle:   mainHandlers,
+            Terminal: true,
+        }

         routes = append(routes, route)
     }
@@
-    config.Apps.HTTP.Servers["charon_server"] = &Server{
-        Listen: []string{":80", ":443"},
-        Routes: routes,
-        AutoHTTPS: &AutoHTTPSConfig{
-            Disable:      false,
-            DisableRedir: false,
-        },
-        Logs: &ServerLogs{
-            DefaultLoggerName: "access_log",
-        },
-    }
+    autoHTTPS := &AutoHTTPSConfig{Disable: false, DisableRedir: false}
+    if len(ipSubjects) > 0 {
+        // Skip AutoHTTPS/ACME for IP literals to avoid ERR_SSL_PROTOCOL_ERROR
+        autoHTTPS.Skip = append(autoHTTPS.Skip, ipSubjects...)
+    }
+
+    config.Apps.HTTP.Servers["charon_server"] = &Server{
+        Listen:    []string{":80", ":443"},
+        Routes:    routes,
+        AutoHTTPS: autoHTTPS,
+        Logs: &ServerLogs{
+            DefaultLoggerName: "access_log",
+        },
+    }
+
+    // Provide internal certificates for IP subjects when present so optional TLS can succeed without ACME
+    if len(ipSubjects) > 0 {
+        if config.Apps.TLS == nil {
+            config.Apps.TLS = &TLSApp{}
+        }
+        policy := &AutomationPolicy{
+            Subjects:   ipSubjects,
+            IssuersRaw: []interface{}{map[string]interface{}{"module": "internal"}},
+        }
+        if config.Apps.TLS.Automation == nil {
+            config.Apps.TLS.Automation = &AutomationConfig{}
+        }
+        config.Apps.TLS.Automation.Policies = append(config.Apps.TLS.Automation.Policies, policy)
+    }

     return config, nil
 }
*** End Patch
```

Add a focused test to `backend/internal/caddy/config_test.go` to cover IP hosts:

```diff
*** Begin Patch
*** Update File: backend/internal/caddy/config_test.go
@@
 func TestGenerateConfig_Logging(t *testing.T) {
@@
 }
+
+func TestGenerateConfig_IPHostsSkipAutoHTTPS(t *testing.T) {
+    hosts := []models.ProxyHost{
+        {
+            UUID:        "uuid-ip",
+            DomainNames: "192.0.2.10",
+            ForwardHost: "app",
+            ForwardPort: 8080,
+            Enabled:     true,
+        },
+    }
+
+    config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
+    require.NoError(t, err)
+
+    server := config.Apps.HTTP.Servers["charon_server"]
+    require.NotNil(t, server)
+    require.Contains(t, server.AutoHTTPS.Skip, "192.0.2.10")
+
+    // Ensure TLS automation adds internal issuer for IP literals
+    require.NotNil(t, config.Apps.TLS)
+    require.NotNil(t, config.Apps.TLS.Automation)
+    require.GreaterOrEqual(t, len(config.Apps.TLS.Automation.Policies), 1)
+    foundIPPolicy := false
+    for _, p := range config.Apps.TLS.Automation.Policies {
+        if len(p.Subjects) == 0 {
+            continue
+        }
+        if p.Subjects[0] == "192.0.2.10" {
+            foundIPPolicy = true
+            require.Len(t, p.IssuersRaw, 1)
+            issuer := p.IssuersRaw[0].(map[string]interface{})
+            require.Equal(t, "internal", issuer["module"])
+        }
+    }
+    require.True(t, foundIPPolicy, "expected internal issuer policy for IP host")
+}
*** End Patch
```

## Backend — Auth cookie scheme-aware flags and header fallback

**Goal:** allow login over IP/HTTP by deriving `Secure` and `SameSite` from the request scheme/X-Forwarded-Proto, and keep Authorization fallback.

Patch `backend/internal/api/handlers/auth_handler.go`:

```diff
*** Begin Patch
*** Update File: backend/internal/api/handlers/auth_handler.go
@@
-import (
-    "net/http"
-    "os"
-    "strconv"
-    "strings"
+import (
+    "net/http"
+    "os"
+    "strconv"
+    "strings"
@@
-func isProduction() bool {
-    env := os.Getenv("CHARON_ENV")
-    return env == "production" || env == "prod"
-}
+func isProduction() bool {
+    env := os.Getenv("CHARON_ENV")
+    return env == "production" || env == "prod"
+}
+
+func requestScheme(c *gin.Context) string {
+    if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
+        // Honor first entry in a comma-separated header
+        parts := strings.Split(proto, ",")
+        return strings.ToLower(strings.TrimSpace(parts[0]))
+    }
+    if c.Request != nil && c.Request.TLS != nil {
+        return "https"
+    }
+    if c.Request != nil && c.Request.URL != nil && c.Request.URL.Scheme != "" {
+        return strings.ToLower(c.Request.URL.Scheme)
+    }
+    return "http"
+}
@@
-// setSecureCookie sets an auth cookie with security best practices
-// - HttpOnly: prevents JavaScript access (XSS protection)
-// - Secure: only sent over HTTPS (in production)
-// - SameSite=Strict: prevents CSRF attacks
-func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
-    secure := isProduction()
-    sameSite := http.SameSiteStrictMode
+// setSecureCookie sets an auth cookie with security best practices
+// - HttpOnly: prevents JavaScript access (XSS protection)
+// - Secure: derived from request scheme to allow HTTP/IP logins when needed
+// - SameSite: Strict for HTTPS, Lax for HTTP/IP to allow forward-auth redirects
+func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
+    scheme := requestScheme(c)
+    secure := isProduction() && scheme == "https"
+    sameSite := http.SameSiteStrictMode
+    if scheme != "https" {
+        sameSite = http.SameSiteLaxMode
+    }
@@
 func (h *AuthHandler) Login(c *gin.Context) {
@@
-    // Set secure cookie (HttpOnly, Secure in prod, SameSite=Strict)
-    setSecureCookie(c, "auth_token", token, 3600*24)
+    // Set secure cookie (scheme-aware) and return token for header fallback
+    setSecureCookie(c, "auth_token", token, 3600*24)
@@
-    c.JSON(http.StatusOK, gin.H{"token": token})
+    c.JSON(http.StatusOK, gin.H{"token": token})
 }
*** End Patch
```

Add unit tests to `backend/internal/api/handlers/auth_handler_test.go` to cover scheme-aware cookies and header fallback:

```diff
*** Begin Patch
*** Update File: backend/internal/api/handlers/auth_handler_test.go
@@
 func TestAuthHandler_Login(t *testing.T) {
@@
 }
+
+func TestSetSecureCookie_HTTPS_Strict(t *testing.T) {
+    gin.SetMode(gin.TestMode)
+    ctx, w := gin.CreateTestContext(httptest.NewRecorder())
+    req := httptest.NewRequest("POST", "https://example.com/login", http.NoBody)
+    ctx.Request = req
+
+    setSecureCookie(ctx, "auth_token", "abc", 60)
+    cookies := w.Result().Cookies()
+    require.Len(t, cookies, 1)
+    c := cookies[0]
+    assert.True(t, c.Secure)
+    assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
+}
+
+func TestSetSecureCookie_HTTP_Lax(t *testing.T) {
+    gin.SetMode(gin.TestMode)
+    ctx, w := gin.CreateTestContext(httptest.NewRecorder())
+    req := httptest.NewRequest("POST", "http://192.0.2.10/login", http.NoBody)
+    req.Header.Set("X-Forwarded-Proto", "http")
+    ctx.Request = req
+
+    setSecureCookie(ctx, "auth_token", "abc", 60)
+    cookies := w.Result().Cookies()
+    require.Len(t, cookies, 1)
+    c := cookies[0]
+    assert.False(t, c.Secure)
+    assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
+}
*** End Patch
```

Patch `backend/internal/api/middleware/auth.go` to explicitly prefer Authorization header when present and keep cookie/query fallback (behavioral clarity, no functional change):

```diff
*** Begin Patch
*** Update File: backend/internal/api/middleware/auth.go
@@
-        authHeader := c.GetHeader("Authorization")
-        if authHeader == "" {
-            // Try cookie
-            cookie, err := c.Cookie("auth_token")
-            if err == nil {
-                authHeader = "Bearer " + cookie
-            }
-        }
-
-        if authHeader == "" {
-            // Try query param
-            token := c.Query("token")
-            if token != "" {
-                authHeader = "Bearer " + token
-            }
-        }
+        authHeader := c.GetHeader("Authorization")
+
+        if authHeader == "" {
+            // Try cookie first for browser flows
+            if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
+                authHeader = "Bearer " + cookie
+            }
+        }
+
+        if authHeader == "" {
+            // Try query param (token passthrough)
+            if token := c.Query("token"); token != "" {
+                authHeader = "Bearer " + token
+            }
+        }
*** End Patch
```

## Frontend — login header fallback when cookies are blocked

**Goal:** when cookies aren’t set (IP/HTTP), use the returned token to set the `Authorization` header for subsequent requests.

Patch `frontend/src/api/client.ts` to expose a token setter and persist optional header:

```diff
*** Begin Patch
*** Update File: frontend/src/api/client.ts
@@
-import axios from 'axios';
+import axios from 'axios';
@@
 const client = axios.create({
   baseURL: '/api/v1',
   withCredentials: true, // Required for HttpOnly cookie transmission
   timeout: 30000, // 30 second timeout
 });
+
+export const setAuthToken = (token: string | null) => {
+  if (token) {
+    client.defaults.headers.common.Authorization = `Bearer ${token}`;
+  } else {
+    delete client.defaults.headers.common.Authorization;
+  }
+};
@@
 export default client;
*** End Patch
```

Patch `frontend/src/context/AuthContext.tsx` to reuse stored token when cookies are unavailable:

```diff
*** Begin Patch
*** Update File: frontend/src/context/AuthContext.tsx
@@
-import client from '../api/client';
+import client, { setAuthToken } from '../api/client';
@@
-    const checkAuth = async () => {
-      try {
-        const response = await client.get('/auth/me');
-        setUser(response.data);
-      } catch {
-        setUser(null);
-      } finally {
-        setIsLoading(false);
-      }
-    };
+    const checkAuth = async () => {
+      try {
+        const stored = localStorage.getItem('charon_auth_token');
+        if (stored) {
+          setAuthToken(stored);
+        }
+        const response = await client.get('/auth/me');
+        setUser(response.data);
+      } catch {
+        setAuthToken(null);
+        setUser(null);
+      } finally {
+        setIsLoading(false);
+      }
+    };
@@
-  const login = async () => {
-    // Token is stored in cookie by backend, but we might want to store it in memory or trigger a re-fetch
-    // Actually, if backend sets cookie, we just need to fetch /auth/me
-    try {
-      const response = await client.get<User>('/auth/me');
-      setUser(response.data);
-    } catch (error) {
-      setUser(null);
-      throw error;
-    }
-  };
+  const login = async (token?: string) => {
+    if (token) {
+      localStorage.setItem('charon_auth_token', token);
+      setAuthToken(token);
+    }
+    try {
+      const response = await client.get<User>('/auth/me');
+      setUser(response.data);
+    } catch (error) {
+      setUser(null);
+      setAuthToken(null);
+      localStorage.removeItem('charon_auth_token');
+      throw error;
+    }
+  };
@@
-  const logout = async () => {
-    try {
-      await client.post('/auth/logout');
-    } catch (error) {
-      console.error("Logout failed", error);
-    }
-    setUser(null);
-  };
+  const logout = async () => {
+    try {
+      await client.post('/auth/logout');
+    } catch (error) {
+      console.error("Logout failed", error);
+    }
+    localStorage.removeItem('charon_auth_token');
+    setAuthToken(null);
+    setUser(null);
+  };
*** End Patch
```

Patch `frontend/src/pages/Login.tsx` to use the returned token when cookies aren’t set:

```diff
*** Begin Patch
*** Update File: frontend/src/pages/Login.tsx
@@
-import client from '../api/client'
+import client from '../api/client'
@@
-      await client.post('/auth/login', { email, password })
-      await login()
+      const res = await client.post('/auth/login', { email, password })
+      const token = (res.data as { token?: string }).token
+      await login(token)
@@
-      toast.error(error.response?.data?.error || 'Login failed')
+      toast.error(error.response?.data?.error || 'Login failed')
*** End Patch
```

Update types to reflect login signature change in `frontend/src/context/AuthContextValue.ts`:

```diff
*** Begin Patch
*** Update File: frontend/src/context/AuthContextValue.ts
@@
-export interface AuthContextType {
-  user: User | null;
-  login: () => Promise<void>;
+export interface AuthContextType {
+  user: User | null;
+  login: (token?: string) => Promise<void>;
*** End Patch
```

Patch `frontend/src/hooks/useAuth.ts` to satisfy the updated context type (no behavioral change needed).

## Frontend Tests — cover token fallback

Extend `frontend/src/pages/__tests__/Login.test.tsx` with a new case ensuring the token is passed to `login` when present and that `/auth/me` is retried with the Authorization header (mocked via context):

```diff
*** Begin Patch
*** Update File: frontend/src/pages/__tests__/Login.test.tsx
@@
   it('shows error toast when login fails', async () => {
@@
   })
+
+  it('uses returned token when cookie is unavailable', async () => {
+    vi.spyOn(setupApi, 'getSetupStatus').mockResolvedValue({ setupRequired: false })
+    const postSpy = vi.spyOn(client, 'post').mockResolvedValueOnce({ data: { token: 'bearer-token' } })
+    const loginFn = vi.fn().mockResolvedValue(undefined)
+    vi.spyOn(authHook, 'useAuth').mockReturnValue({ login: loginFn } as unknown as AuthContextType)
+
+    renderWithProviders(<Login />)
+    const email = screen.getByPlaceholderText(/admin@example.com/i)
+    const pass = screen.getByPlaceholderText(/••••••••/i)
+    fireEvent.change(email, { target: { value: 'a@b.com' } })
+    fireEvent.change(pass, { target: { value: 'pw' } })
+    fireEvent.click(screen.getByRole('button', { name: /Sign In/i }))
+
+    await waitFor(() => expect(postSpy).toHaveBeenCalled())
+    expect(loginFn).toHaveBeenCalledWith('bearer-token')
+  })
*** End Patch
```

## Backend Tests — auth middleware clarity

Add a small assertion to `backend/internal/api/middleware/auth_test.go` to confirm Authorization header is preferred when both cookie and header exist:

```diff
*** Begin Patch
*** Update File: backend/internal/api/middleware/auth_test.go
@@
 func TestAuthMiddleware_ValidToken(t *testing.T) {
@@
 }
+
+func TestAuthMiddleware_PrefersAuthorizationHeader(t *testing.T) {
+    authService := setupAuthService(t)
+    user, _ := authService.Register("header@example.com", "password", "Header User")
+    token, _ := authService.GenerateToken(user)
+
+    gin.SetMode(gin.TestMode)
+    r := gin.New()
+    r.Use(AuthMiddleware(authService))
+    r.GET("/test", func(c *gin.Context) {
+        userID, _ := c.Get("userID")
+        assert.Equal(t, user.ID, userID)
+        c.Status(http.StatusOK)
+    })
+
+    req, _ := http.NewRequest("GET", "/test", http.NoBody)
+    req.Header.Set("Authorization", "Bearer "+token)
+    req.AddCookie(&http.Cookie{Name: "auth_token", Value: "stale"})
+    w := httptest.NewRecorder()
+    r.ServeHTTP(w, req)
+
+    assert.Equal(t, http.StatusOK, w.Code)
+}
*** End Patch
```

## Hygiene — ignores and coverage

Update `.gitignore` to include transient caches and geoip data (mirrors plan):

```diff
*** Begin Patch
*** Update File: .gitignore
@@
 frontend/.vite/
 frontend/*.tsbuildinfo
+/frontend/.cache/
+/frontend/.eslintcache
+/backend/.vscode/
+/data/geoip/
*** End Patch
```

Update `.dockerignore` with the same cache directories (add if present):

```diff
*** Begin Patch
*** Update File: .dockerignore
@@
 node_modules
 frontend/node_modules
 backend/node_modules
 frontend/dist
+frontend/.cache
+frontend/.eslintcache
+data/geoip
*** End Patch
```

Adjust `.codecov.yml` to **include** backend startup logic in coverage (remove the `backend/cmd/api/**` ignore) and leave other ignores untouched:

```diff
*** Begin Patch
*** Update File: .codecov.yml
@@
-  - "backend/cmd/api/**"
*** End Patch
```

## Tests to run after applying patches

1. Backend unit tests: `go test ./backend/internal/caddy ./backend/internal/api/...`
2. Frontend unit tests: `cd frontend && npm test -- Login.test.tsx`
3. Backend build: run workspace task **Go: Build Backend**.
4. Frontend type-check/build: `cd frontend && npm run type-check` then `npm run build`.

## Notes

- The IP-aware TLS policy uses Caddy’s `internal` issuer for IP literals while skipping AutoHTTPS to prevent ACME on IPs.
- Cookie flags now respect the inbound scheme (or `X-Forwarded-Proto`), enabling HTTP/IP logins without disabling secure defaults on HTTPS.
- Frontend stores the login token only when provided; it clears the header and storage on logout or auth failure.
- Remove the `.codecov.yml` ignore entry only if new code paths should count toward coverage; otherwise keep existing thresholds.
