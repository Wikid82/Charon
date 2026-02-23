# Docker Socket Trace Analysis

**Date**: 2025-12-22
**Issue**: Creating a new proxy host using the local docker socket fails with 503 (previously 500)
**Status**: Root cause identified

---

## Executive Summary

**ROOT CAUSE**: The container runs as non-root user `charon` (uid=1000, gid=1000), but the Docker socket mounted into the container is owned by `root:docker` (gid=988 on host). The `charon` user is not a member of the `docker` group, so socket access is denied with `Permission denied`.

**The 503 is correct behavior** - it accurately reflects that Docker is unavailable due to permission restrictions. The error handling code change from 500 to 503 was an improvement, not a bug.

---

## 1. Full Workflow Trace

### Frontend Layer

#### A. ProxyHostForm Component

- **File**: [frontend/src/components/ProxyHostForm.tsx](../../frontend/src/components/ProxyHostForm.tsx)
- **State**: `connectionSource` - defaults to `'custom'`, can be `'local'` or a remote server UUID
- **Hook invocation** (line ~146):

  ```typescript
  const { containers: dockerContainers, isLoading: dockerLoading, error: dockerError } = useDocker(
    connectionSource === 'local' ? 'local' : undefined,
    connectionSource !== 'local' && connectionSource !== 'custom' ? connectionSource : undefined
  )
  ```

- **Error display** (line ~361):

  ```typescript
  {dockerError && connectionSource !== 'custom' && (
    <p className="text-xs text-red-400 mt-1">
      Failed to connect: {(dockerError as Error).message}
    </p>
  )}
  ```

#### B. useDocker Hook

- **File**: [frontend/src/hooks/useDocker.ts](../../frontend/src/hooks/useDocker.ts)
- **Function**: `useDocker(host?: string | null, serverId?: string | null)`
- **Query configuration**:

  ```typescript
  useQuery({
    queryKey: ['docker-containers', host, serverId],
    queryFn: () => dockerApi.listContainers(host || undefined, serverId || undefined),
    enabled: Boolean(host) || Boolean(serverId),
    retry: 1,
  })
  ```

- When `connectionSource === 'local'`, calls `dockerApi.listContainers('local', undefined)`

#### C. Docker API Client

- **File**: [frontend/src/api/docker.ts](../../frontend/src/api/docker.ts)
- **Function**: `dockerApi.listContainers(host?: string, serverId?: string)`
- **Request**: `GET /api/v1/docker/containers?host=local`
- **Response type**: `DockerContainer[]`

---

### Backend Layer

#### D. Routes Registration

- **File**: [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go)
- **Registration** (lines 199-204):

  ```go
  dockerService, err := services.NewDockerService()
  if err == nil { // Only register if Docker is available
      dockerHandler := handlers.NewDockerHandler(dockerService, remoteServerService)
      dockerHandler.RegisterRoutes(protected)
  } else {
      logger.Log().WithError(err).Warn("Docker service unavailable")
  }
  ```

- **CRITICAL**: Docker routes only register if `NewDockerService()` succeeds (client construction, not socket access)
- Route: `GET /api/v1/docker/containers` (protected, requires auth)

#### E. Docker Handler

- **File**: [backend/internal/api/handlers/docker_handler.go](../../backend/internal/api/handlers/docker_handler.go)
- **Function**: `ListContainers(c *gin.Context)`
- **Input validation** (SSRF hardening):

  ```go
  host := strings.TrimSpace(c.Query("host"))
  serverID := strings.TrimSpace(c.Query("server_id"))

  // SSRF hardening: only allow "local" or empty
  if host != "" && host != "local" {
      c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid docker host selector"})
      return
  }
  ```

- **Service call**: `h.dockerService.ListContainers(c.Request.Context(), host)`
- **Error handling** (lines 60-69):

  ```go
  if err != nil {
      var unavailableErr *services.DockerUnavailableError
      if errors.As(err, &unavailableErr) {
          c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Docker daemon unavailable"})  // 503
          return
      }
      c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list containers"})  // 500
      return
  }
  ```

#### F. Docker Service

- **File**: [backend/internal/services/docker_service.go](../../backend/internal/services/docker_service.go)
- **Constructor**: `NewDockerService()`

  ```go
  cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
  ```

  - Uses `client.FromEnv` which reads `DOCKER_HOST` env var (defaults to `unix:///var/run/docker.sock`)
  - **Does NOT verify socket access** - only constructs client object

- **Function**: `ListContainers(ctx context.Context, host string)`

  ```go
  if host == "" || host == "local" {
      cli = s.client  // Use default local client
  }
  containers, err := cli.ContainerList(ctx, container.ListOptions{All: false})
  if err != nil {
      if isDockerConnectivityError(err) {
          return nil, &DockerUnavailableError{err: err}  // Triggers 503
      }
      return nil, fmt.Errorf("failed to list containers: %w", err)  // Triggers 500
  }
  ```

- **Error detection**: `isDockerConnectivityError(err)` (lines 104-152)
  - Checks for: "cannot connect to docker daemon", "is the docker daemon running", timeout errors
  - Checks syscall errors: `ENOENT`, `EACCES`, `EPERM`, `ECONNREFUSED`
  - **Matches `syscall.EACCES` (permission denied)** → returns `DockerUnavailableError` → **503**

---

## 2. Request/Response Shapes

### Frontend → Backend Request

```
GET /api/v1/docker/containers?host=local
Authorization: Bearer <jwt_token>
```

### Backend → Frontend Response (Success - 200)

```json
[
  {
    "id": "abc123def456",
    "names": ["my-container"],
    "image": "nginx:latest",
    "state": "running",
    "status": "Up 2 hours",
    "network": "bridge",
    "ip": "172.17.0.2",
    "ports": [{"private_port": 80, "public_port": 8080, "type": "tcp"}]
  }
]
```

### Backend → Frontend Response (Error - 503)

```json
{
  "error": "Docker daemon unavailable"
}
```

---

## 3. Error Conditions Triggering 503

The 503 `Service Unavailable` is returned when `isDockerConnectivityError()` returns `true`:

| Condition | Check in Code | Matches Our Case |
|-----------|---------------|------------------|
| Socket missing | `syscall.ENOENT` or `os.ErrNotExist` | No |
| Permission denied | `syscall.EACCES` or `syscall.EPERM` | **YES** ✓ |
| Connection refused | `syscall.ECONNREFUSED` | No |
| Timeout | `net.Error.Timeout()` or `context.DeadlineExceeded` | No |
| Daemon not running | String contains "cannot connect" / "daemon running" | No |

---

## 4. Docker Configuration Analysis

### Dockerfile

- **File**: [Dockerfile](../../Dockerfile)
- **User creation** (lines 154-156):

  ```dockerfile
  RUN addgroup -g 1000 charon && \
      adduser -D -u 1000 -G charon -h /app -s /sbin/nologin charon
  ```

- **Runtime user** (line 286):

  ```dockerfile
  USER charon
  ```

- **Result**: Container runs as `uid=1000, gid=1000` (charon:charon)

### Docker Compose Files

All compose files mount the socket identically:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
```

| File | Mount Present |
|------|---------------|
| [.docker/compose/docker-compose.yml](../../.docker/compose/docker-compose.yml) | ✓ |
| [.docker/compose/docker-compose.local.yml](../../.docker/compose/docker-compose.local.yml) | ✓ |
| [.docker/compose/docker-compose.dev.yml](../../.docker/compose/docker-compose.dev.yml) | ✓ |
| [docker-compose.test.yml](../../docker-compose.test.yml) | ✓ |

### Runtime Verification (from live container)

```bash
# Socket exists inside container
$ ls -la /var/run/docker.sock
srw-rw----    1 root     988     0 Dec 12 22:40 /var/run/docker.sock

# Container user identity
$ id
uid=1000(charon) gid=1000(charon) groups=1000(charon)

# Direct socket access test
$ curl --unix-socket /var/run/docker.sock http://localhost/containers/json
# Returns: exit code 7 (connection refused due to permission denied)

# Explicit permission check
$ cat /var/run/docker.sock
cat: can't open '/var/run/docker.sock': Permission denied
```

### Host System

```bash
$ getent group 988
docker:x:988:

$ stat -c '%U:%G' /var/run/docker.sock
root:docker
```

---

## 5. Root Cause Analysis

### The Permission Gap

| Component | Value |
|-----------|-------|
| Socket owner | `root:docker` (gid=988) |
| Socket permissions | `srw-rw----` (660) |
| Container user | `charon` (uid=1000, gid=1000) |
| Container groups | Only `charon` (1000) |
| Docker group in container | **Does not exist** |

**The `charon` user cannot access the socket because:**

1. Not owner (not root)
2. Not in the socket's group (gid=988 doesn't exist in container, and charon isn't in it)
3. No "other" permissions on socket

### Why This Happens

The Docker socket's group ID (988 on this host) is a **host-specific value**. Different systems assign different GIDs to the `docker` group:

- Debian/Ubuntu: often 999 or 998
- Alpine: often 101 (from `docker` package)
- RHEL/CentOS: varies
- This host: 988

The container has no knowledge of the host's group mappings. When the socket is mounted, it retains the host's numeric GID, but the container has no group with that GID.

---

## 6. Why 503 (Not 500) Is Correct

The error mapping change that returned 503 instead of 500 was **correct and intentional**:

- **500 Internal Server Error**: Indicates a bug or unexpected failure in the application
- **503 Service Unavailable**: Indicates the requested service is temporarily unavailable due to external factors

Docker being inaccessible due to socket permissions is an **environmental/configuration issue**, not an application bug. The 503 correctly signals:

1. The API endpoint is working
2. The underlying Docker service is unavailable
3. The issue is likely external (deployment configuration)

---

## 7. Solutions

### Option A: Run Container as Root (Not Recommended)

Remove `USER charon` from Dockerfile. Breaks security best practices (CIS Docker Benchmark 4.1).

### Option B: Add Docker Group to Container at Build Time

```dockerfile
# Problem: GID varies by host system
RUN addgroup -g 988 docker && adduser charon docker
```

**Issue**: Assumes host Docker GID is 988; breaks on other systems.

### Option C: Dynamic Group Assignment at Runtime (Recommended)

Modify entrypoint to detect and add the socket's group:

```bash
# In docker-entrypoint.sh, before starting the app:
if [ -S /var/run/docker.sock ]; then
    DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)
    if ! getent group "$DOCKER_GID" >/dev/null 2>&1; then
        # Create a group with the socket's GID
        addgroup -g "$DOCKER_GID" docker 2>/dev/null || true
    fi
    # Add charon user to the docker group
    adduser charon docker 2>/dev/null || true
fi
```

**Issue**: Requires container to start as root, then drop privileges.

### Option D: Use DOCKER_HOST Environment Variable

Allow users to specify an alternative Docker endpoint (TCP, SSH, or different socket path):

```yaml
environment:
  - DOCKER_HOST=tcp://host.docker.internal:2375
```

**Issue**: Requires exposing Docker API over network (security implications).

### Option E: Document User Requirement (Workaround)

Add documentation requiring users to either:

1. Run the container with `--user root` (not recommended)
2. Change socket permissions on host: `chmod 666 /var/run/docker.sock` (security risk)
3. Accept that Docker integration is unavailable when running as non-root

---

## 8. Recommendations

### Immediate (No Code Change)

1. **Update documentation** to explain the permission requirement
2. **Add health check** for Docker availability in the UI (show "Docker integration unavailable" gracefully)

### Short Term

1. **Add startup warning log** when Docker socket is inaccessible:

   ```go
   // In routes.go or docker_service.go
   if _, err := cli.Ping(ctx); err != nil {
       logger.Log().Warn("Docker socket inaccessible - container discovery disabled")
   }
   ```

### Medium Term

1. **Implement Option C** with proper privilege dropping
2. **Add environment variable** `CHARON_DOCKER_ENABLED=false` to explicitly disable Docker integration

### Long Term

1. Consider **podman socket** compatibility
2. Consider **Docker SDK over TCP** as alternative

---

## 9. Conclusion

The 503 error is **working as designed**. The Docker socket permission model fundamentally conflicts with running containers as non-root users unless explicit configuration is done at deployment time.

**The fix is not in the code, but in deployment configuration or documentation.**
