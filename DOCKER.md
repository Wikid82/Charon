# Docker Deployment Guide

CaddyProxyManager+ is designed for Docker-first deployment, making it easy for home users to run Caddy without learning Caddyfile syntax.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/Wikid82/CaddyProxyManagerPlus.git
cd CaddyProxyManagerPlus

# Start the stack
docker-compose up -d

# Access the UI
open http://localhost:8080
```

## Architecture

The Docker stack consists of two services:

1. **app** (`caddyproxymanager-plus`): Management interface
   - Manages proxy host configuration
   - Provides web UI on port 8080
   - Communicates with Caddy via admin API

2. **caddy**: Reverse proxy server
   - Handles incoming traffic on ports 80/443
   - Automatic HTTPS with Let's Encrypt
   - Configured dynamically via JSON API

```
┌──────────────┐
│   Internet   │
└──────┬───────┘
       │ :80, :443
       ▼
┌──────────────┐     Admin API      ┌──────────────┐
│    Caddy     │◄───────:2019───────┤  CPM+ App    │
│ (Proxy)      │                    │  (Manager)   │
└──────┬───────┘                    └──────┬───────┘
       │                                   │
       ▼                                   ▼
  Your Services                       :8080 (Web UI)
```

## Environment Variables

Configure CPM+ via environment variables in `docker-compose.yml`:

```yaml
environment:
  - CPM_ENV=production              # production | development
  - CPM_HTTP_PORT=8080              # Management UI port
  - CPM_DB_PATH=/app/data/cpm.db    # SQLite database location
  - CPM_CADDY_ADMIN_API=http://caddy:2019  # Caddy admin endpoint
  - CPM_CADDY_CONFIG_DIR=/app/data/caddy   # Config snapshots
```

## Volumes

Three persistent volumes store your data:

- **app_data**: CPM+ database, config snapshots, logs
- **caddy_data**: Caddy certificates, ACME account data
- **caddy_config**: Caddy runtime configuration

To backup your configuration:

```bash
# Backup volumes
docker run --rm -v cpm_app_data:/data -v $(pwd):/backup alpine tar czf /backup/cpm-backup.tar.gz /data

# Restore from backup
docker run --rm -v cpm_app_data:/data -v $(pwd):/backup alpine tar xzf /backup/cpm-backup.tar.gz -C /
```

## Ports

Default port mapping:

- **80**: HTTP (Caddy) - redirects to HTTPS
- **443/tcp**: HTTPS (Caddy)
- **443/udp**: HTTP/3 (Caddy)
- **8080**: Management UI (CPM+)
- **2019**: Caddy admin API (internal only, exposed in dev mode)

## Development Mode

Development mode exposes the Caddy admin API externally for debugging:

```bash
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

Access Caddy admin API: `http://localhost:2019/config/`

## Health Checks

CPM+ includes a health check endpoint:

```bash
# Check if app is running
curl http://localhost:8080/api/v1/health

# Check Caddy status
docker-compose exec caddy caddy version
```

## Troubleshooting

### App can't reach Caddy

**Symptom**: "Caddy unreachable" errors in logs

**Solution**: Ensure both containers are on the same network:
```bash
docker-compose ps  # Check both services are "Up"
docker-compose logs caddy  # Check Caddy logs
```

### Certificates not working

**Symptom**: HTTP works but HTTPS fails

**Check**:
1. Port 80/443 are accessible from the internet
2. DNS points to your server
3. Caddy logs: `docker-compose logs caddy | grep -i acme`

### Config changes not applied

**Symptom**: Changes in UI don't affect routing

**Debug**:
```bash
# View current Caddy config
curl http://localhost:2019/config/ | jq

# Check CPM+ logs
docker-compose logs app

# Manual config reload
curl -X POST http://localhost:8080/api/v1/caddy/reload
```

## Updating

Pull the latest images and restart:

```bash
docker-compose pull
docker-compose up -d
```

For specific versions:

```bash
# Edit docker-compose.yml to pin version
image: ghcr.io/wikid82/caddyproxymanagerplus:v1.0.0

docker-compose up -d
```

## Building from Source

```bash
# Build multi-arch images
docker buildx build --platform linux/amd64,linux/arm64 -t caddyproxymanager-plus:local .

# Or use Make
make docker-build
```

## Security Considerations

1. **Caddy admin API**: Keep port 2019 internal (not exposed in production compose)
2. **Management UI**: Add authentication (Issue #7) before exposing to internet
3. **Certificates**: Caddy stores private keys in `caddy_data` - protect this volume
4. **Database**: SQLite file contains all config - backup regularly

## Integration with Existing Caddy

If you already have Caddy running, you can point CPM+ to it:

```yaml
environment:
  - CPM_CADDY_ADMIN_API=http://your-caddy-host:2019
```

**Warning**: CPM+ will replace Caddy's entire configuration. Backup first!

## Platform-Specific Notes

### Synology NAS

Use Container Manager (Docker GUI):
1. Import `docker-compose.yml`
2. Map port 80/443 to your NAS IP
3. Enable auto-restart

### Unraid

1. Use Docker Compose Manager plugin
2. Add compose file to `/boot/config/plugins/compose.manager/projects/cpm/`
3. Start via web UI

### Home Assistant Add-on

Coming soon in Beta release.

## Performance Tuning

For high-traffic deployments:

```yaml
# docker-compose.yml
services:
  caddy:
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M
```

## Next Steps

- Configure your first proxy host via UI
- Enable automatic HTTPS (happens automatically)
- Add authentication (Issue #7)
- Integrate CrowdSec (Issue #15)
