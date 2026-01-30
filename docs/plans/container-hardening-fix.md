# Container Hardening Configuration Fix Plan

**Issue**: Documentation shows conflicting container hardening configuration with `read_only: true` and volume mounts that may not work correctly.

**Date**: 2025-12-23

## Research Findings: Where Charon Writes Data

### 1. Primary Data Directories (Evidence from code analysis)

#### `/app/data/` - Main persistent data directory

- **Database**: `/app/data/charon.db` (default, configurable via `CHARON_DB_PATH`)
  - Source: `backend/internal/config/config.go:44`
  - SQLite database file with WAL mode
  - Requires write access for database operations

- **Database Backups**: `/app/data/backups/`
  - Source: `backend/internal/services/backup_service.go:35`
  - Creates timestamped `.zip` backups of database and Caddy data
  - Cron job runs daily at 3 AM
  - Requires write access for backup creation and cleanup

- **Caddy Configuration**: `/app/data/caddy/`
  - Source: `backend/internal/config/config.go:47`
  - Stores Caddy TLS certificates (Let's Encrypt, ZeroSSL, custom)
  - Certificate storage under subdirectories like `certificates/acme-v02.api.letsencrypt.org-directory/`
  - Requires persistent write access for certificate management

- **Import Directory**: `/app/data/imports/`
  - Source: `backend/internal/config/config.go:50`
  - Used for Caddyfile import functionality
  - Requires write access for import operations

- **CrowdSec Data**: `/app/data/crowdsec/`
  - Source: `backend/internal/config/config.go:57` and `.docker/docker-entrypoint.sh:96`
  - Persistent CrowdSec configuration and data
  - `/app/data/crowdsec/config/` - CrowdSec configuration files (symlinked to `/etc/crowdsec`)
  - `/app/data/crowdsec/data/` - CrowdSec database and persistent data
  - `/app/data/crowdsec/hub_cache/` - Hub item cache
  - Requires write access for CrowdSec operation

- **GeoIP Database**: `/app/data/geoip/GeoLite2-Country.mmdb`
  - Source: `Dockerfile:255` (downloaded at build time)
  - Read-only at runtime (pre-populated in image)
  - No write access needed at runtime

#### `/var/log/` - Log files (Requires tmpfs for read-only root)

- **Caddy Logs**: `/var/log/caddy/access.log`
  - Source: `backend/internal/caddy/config.go:18` and `configs/crowdsec/acquis.yaml`
  - JSON-formatted access logs for HTTP/HTTPS traffic
  - CrowdSec monitors this file when enabled
  - Requires write access with log rotation

- **CrowdSec Logs**: `/var/log/crowdsec/`
  - Source: `.docker/docker-entrypoint.sh:100`
  - CrowdSec agent and LAPI logs
  - Requires write access when CrowdSec is enabled

#### `/config/` - Caddy runtime configuration (Requires tmpfs for read-only root)

- **Caddy JSON Config**: `/config/caddy.json`
  - Source: `.docker/docker-entrypoint.sh:203`
  - Runtime Caddy configuration loaded via Admin API
  - Generated dynamically by Charon backend
  - Requires write access for configuration updates

#### `/tmp/` - Temporary files (Requires tmpfs for read-only root)

- Used by CrowdSec hub operations
  - Source: Various test files show `/tmp/buildenv_*` patterns for hub sync
  - Requires write access for temporary file operations

#### `/etc/crowdsec` - Symlink to persistent storage

- **Symlink**: `/etc/crowdsec -> /app/data/crowdsec/config`
  - Source: `Dockerfile:405` and `.docker/docker-entrypoint.sh:110`
  - Created at build time (as root) to allow persistent CrowdSec config
  - Symlink itself is read-only at runtime
  - Target directory requires write access

#### `/var/lib/crowdsec` - CrowdSec data directory (May require tmpfs)

- **CrowdSec Runtime Data**: `/var/lib/crowdsec/data/`
  - Source: `Dockerfile:251` and `.docker/docker-entrypoint.sh:110`
  - CrowdSec agent runtime data
  - May be symlinked or used for temporary data
  - Investigate if this can be redirected to `/app/data/crowdsec/data`

### 2. Docker Socket Access

- **Docker Socket**: `/var/run/docker.sock` (read-only mount)
  - Source: `.docker/compose/docker-compose.yml:32`
  - Used for container discovery feature
  - Read-only access is sufficient

## Current Docker Configuration Analysis

### Volume Mounts in Production (`docker-compose.yml`)

```yaml
volumes:
  - cpm_data:/app/data           # Persistent database, caddy certs, backups
  - caddy_data:/data             # Caddy data directory (may be unused?)
  - caddy_config:/config         # Caddy runtime config (needs write)
  - crowdsec_data:/app/data/crowdsec  # CrowdSec persistent data
  - /var/run/docker.sock:/var/run/docker.sock:ro  # Docker integration
```

### Issues with Current Setup

1. **`caddy_data:/data`** - This volume may be unnecessary as Caddy uses `/app/data/caddy/` for certificates
2. **`/config` mount** - Required but may conflict with `read_only: true` root filesystem
3. **`/var/log/`** - Not mounted as tmpfs, will fail with `read_only: true`
4. **`/tmp/`** - Not mounted as tmpfs, will fail with `read_only: true`
5. **`/var/lib/crowdsec`** - May need tmpfs or redirection to `/app/data/crowdsec/data`

## Correct Container Hardening Configuration

### Strategy

1. **Root filesystem**: `read_only: true` for security
2. **Persistent data**: Named volume at `/app/data` for all persistent data
3. **Ephemeral data**: tmpfs mounts for logs, temp files, and runtime config
4. **Symlinks**: Leverage existing symlinks in the image (`/etc/crowdsec -> /app/data/crowdsec/config`)

### Recommended Docker Compose Configuration

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped

    # Security: Read-only root filesystem
    read_only: true

    # Drop all capabilities except NET_BIND_SERVICE (for ports 80/443)
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE

    # Prevent privilege escalation
    security_opt:
      - no-new-privileges:true

    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
      - "8080:8080"

    environment:
      - CHARON_ENV=production
      - TZ=UTC
      - CHARON_HTTP_PORT=8080
      - CHARON_DB_PATH=/app/data/charon.db
      - CHARON_FRONTEND_DIR=/app/frontend/dist
      - CHARON_CADDY_ADMIN_API=http://localhost:2019
      - CHARON_CADDY_CONFIG_DIR=/app/data/caddy
      - CHARON_CADDY_BINARY=caddy
      - CHARON_IMPORT_CADDYFILE=/import/Caddyfile
      - CHARON_IMPORT_DIR=/app/data/imports
      - CHARON_CROWDSEC_CONFIG_DIR=/app/data/crowdsec

    extra_hosts:
      - "host.docker.internal:host-gateway"

    volumes:
      # Persistent data (database, certificates, backups, CrowdSec config)
      - charon_data:/app/data

      # Ephemeral tmpfs mounts for writable directories
      - type: tmpfs
        target: /tmp
        tmpfs:
          size: 100M
          mode: 1777  # Sticky bit for multi-user temp directory

      - type: tmpfs
        target: /var/log/caddy
        tmpfs:
          size: 100M
          mode: 0755

      - type: tmpfs
        target: /var/log/crowdsec
        tmpfs:
          size: 100M
          mode: 0755

      - type: tmpfs
        target: /config
        tmpfs:
          size: 10M
          mode: 0755

      - type: tmpfs
        target: /var/lib/crowdsec
        tmpfs:
          size: 50M
          mode: 0755

      - type: tmpfs
        target: /run
        tmpfs:
          size: 10M
          mode: 0755

      # Docker socket for container discovery (read-only)
      - /var/run/docker.sock:/var/run/docker.sock:ro

      # Optional: Import existing Caddyfile (read-only)
      # - ./my-existing-Caddyfile:/import/Caddyfile:ro

    healthcheck:
      test: ["CMD", "curl", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  charon_data:
    driver: local
```

### Why This Configuration Works

1. **`read_only: true`** - Root filesystem is read-only, preventing unauthorized file modifications
2. **`/app/data` volume** - All persistent data in one place:
   - Database (`charon.db`)
   - Backups (`backups/`)
   - Caddy certificates (`caddy/certificates/`)
   - Import directory (`imports/`)
   - CrowdSec config and data (`crowdsec/`)
   - GeoIP database (pre-populated in image, read-only)
3. **tmpfs mounts** - Ephemeral writable directories:
   - `/tmp` - Temporary files (CrowdSec hub operations)
   - `/var/log/caddy` - Caddy access logs (monitored by CrowdSec)
   - `/var/log/crowdsec` - CrowdSec agent logs
   - `/config` - Caddy runtime JSON config
   - `/var/lib/crowdsec` - CrowdSec runtime data
   - `/run` - Runtime state files and PIDs
4. **Symlinks preserved** - `/etc/crowdsec -> /app/data/crowdsec/config` still works
5. **Capabilities dropped** - Only NET_BIND_SERVICE retained for ports 80/443
6. **No privilege escalation** - `no-new-privileges:true` prevents privilege escalation attacks

### What About the Removed Volumes?

- **`caddy_data:/data`** - Removed, not used by Charon (Caddy stores certs in `/app/data/caddy/`)
- **`caddy_config:/config`** - Replaced with tmpfs (runtime config doesn't need persistence)
- **`crowdsec_data:/app/data/crowdsec`** - Merged into main `charon_data` volume

## Validation Checklist

Before deploying this configuration, validate:

- [ ] Charon starts successfully with `read_only: true`
- [ ] Database operations work (create/read/update/delete)
- [ ] Caddy can obtain and renew TLS certificates
- [ ] Backups are created and restored successfully
- [ ] CrowdSec can start, update hub, and process logs (if enabled)
- [ ] Log files are written to `/var/log/caddy/access.log`
- [ ] Container discovery works with Docker socket
- [ ] Import directory accepts uploaded Caddyfiles
- [ ] No "read-only filesystem" errors in logs

## Testing Steps

1. **Deploy with hardening**:

   ```bash
   docker compose -f .docker/compose/docker-compose.yml down
   # Update docker-compose.yml with new configuration
   docker compose -f .docker/compose/docker-compose.yml up -d
   ```

2. **Check startup logs**:

   ```bash
   docker logs charon
   ```

3. **Test database operations**:
   - Create a new proxy host via UI
   - Edit an existing proxy host
   - Delete a proxy host
   - Check `/app/data/charon.db` is updated

4. **Test certificate management**:
   - Add a domain with Let's Encrypt
   - Verify certificate is obtained
   - Check `/app/data/caddy/certificates/` for certificate files

5. **Test backups**:
   - Trigger manual backup via UI
   - Verify backup appears in `/app/data/backups/`
   - Test backup restoration

6. **Test CrowdSec** (if enabled):
   - Enable CrowdSec via Security dashboard
   - Check CrowdSec starts successfully
   - Verify hub items are installed
   - Check logs are being processed

7. **Inspect filesystem permissions**:

   ```bash
   docker exec charon ls -la /app/data
   docker exec charon ls -la /var/log/caddy
   docker exec charon ls -la /config
   docker exec charon ls -la /tmp
   ```

## Rollback Plan

If the hardened configuration causes issues:

1. Remove `read_only: true` from docker-compose.yml
2. Revert to previous volume configuration
3. Restart container: `docker compose restart charon`

## Implementation Plan

1. **Update documentation** in relevant files:
   - Update container hardening example with correct configuration
   - Add explanation of why each mount is needed
   - Document tmpfs size recommendations

2. **Create example file**: `examples/docker-compose.hardened.yml`
   - Full working example with read-only root filesystem
   - Comments explaining each security feature
   - Reference from main documentation

3. **Add to troubleshooting docs**:
   - Document common issues with read-only root filesystem
   - Explain tmpfs sizing considerations
   - Provide tips for debugging permission issues

4. **Test with integration tests**:
   - Add integration test that deploys hardened configuration
   - Verify all features work correctly
   - Include in CI/CD pipeline

## References

- **CIS Docker Benchmark 5.12**: Run containers with a read-only root filesystem
- **CIS Docker Benchmark 5.25**: Restrict container from acquiring additional privileges
- **CIS Docker Benchmark 5.26**: Ensure container health is checked at runtime
- **Code Evidence**:
  - Database path: `backend/internal/config/config.go:44`
  - Backup service: `backend/internal/services/backup_service.go:35`
  - Caddy config: `backend/internal/caddy/config.go:18`
  - CrowdSec setup: `.docker/docker-entrypoint.sh:96-206`
  - Volume mounts: `.docker/compose/docker-compose.yml:30-36`
