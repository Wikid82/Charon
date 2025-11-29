# Docker Deployment Guide

Charon is designed for Docker-first deployment, making it easy for home users to run Caddy without learning Caddyfile syntax.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/Wikid82/charon.git
cd charon

# Start the stack
docker-compose up -d

# Access the UI
open http://localhost:8080
```

## Architecture

Charon runs as a **single container** that includes:
1.  **Caddy Server**: The reverse proxy engine (ports 80/443).
2.  **Charon Backend**: The Go API that manages Caddy via its API (binary: `charon`, `cpmp` symlink preserved).
3.  **Charon Frontend**: The React web interface (port 8080).

This unified architecture simplifies deployment, updates, and data management.

```
┌──────────────────────────────────────────┐
│  Container (charon / cpmp)               │
│                                          │
│  ┌──────────┐   API    ┌──────────────┐  │
│  │  Caddy   │◄──:2019──┤  CPM+ App    │  │
│  │ (Proxy)  │          │  (Manager)   │  │
│  └────┬─────┘          └──────┬───────┘  │
│       │                       │          │
└───────┼───────────────────────┼──────────┘
        │ :80, :443             │ :8080
        ▼                       ▼
    Internet                 Web UI
```

## Configuration

### Volumes

Persist your data by mounting these volumes:

| Host Path | Container Path | Description |
|-----------|----------------|-------------|
| `./data` | `/app/data` | **Critical**. Stores the SQLite database (default `charon.db`, `cpm.db` fallback) and application logs. |
| `./caddy_data` | `/data` | **Critical**. Stores Caddy's SSL certificates and keys. |
| `./caddy_config` | `/config` | Stores Caddy's autosave configuration. |

### Environment Variables

Configure the application via `docker-compose.yml`:

| Variable | Default | Description |
|----------|---------|-------------|
| `CHARON_ENV` | `production` | Set to `development` for verbose logging (`CPM_ENV` supported for backward compatibility). |
| `CHARON_HTTP_PORT` | `8080` | Port for the Web UI (`CPM_HTTP_PORT` supported for backward compatibility). |
| `CHARON_DB_PATH` | `/app/data/charon.db` | Path to the SQLite database (`CPM_DB_PATH` supported for backward compatibility). |
| `CHARON_CADDY_ADMIN_API` | `http://localhost:2019` | Internal URL for Caddy API (`CPM_CADDY_ADMIN_API` supported for backward compatibility). |

## NAS Deployment Guides

### Synology (Container Manager / Docker)

1.  **Prepare Folders**: Create a folder `docker/charon` (or `docker/cpmp` for backward compatibility) and subfolders `data`, `caddy_data`, and `caddy_config`.
2.  **Download Image**: Search for `ghcr.io/wikid82/charon` in the Registry and download the `latest` tag.
3.  **Launch Container**:
    *   **Network**: Use `Host` mode (recommended for Caddy to see real client IPs) OR bridge mode mapping ports `80:80`, `443:443`, and `8080:8080`.
    *   **Volume Settings**:
        *   `/docker/charon/data` -> `/app/data` (or `/docker/cpmp/data` -> `/app/data` for backward compatibility)
        *   `/docker/charon/caddy_data` -> `/data` (or `/docker/cpmp/caddy_data` -> `/data` for backward compatibility)
        *   `/docker/charon/caddy_config` -> `/config` (or `/docker/cpmp/caddy_config` -> `/config` for backward compatibility)
    *   **Environment**: Add `CHARON_ENV=production` (or `CPM_ENV=production` for backward compatibility).
4.  **Finish**: Start the container and access `http://YOUR_NAS_IP:8080`.

### Unraid

1.  **Community Apps**: (Coming Soon) Search for "charon".
2.  **Manual Install**:
    *   Click **Add Container**.
    *   **Name**: Charon
    *   **Repository**: `ghcr.io/wikid82/charon:latest`
    *   **Network Type**: Bridge
    *   **WebUI**: `http://[IP]:[PORT:8080]`
    *   **Port mappings**:
        *   Container Port: `80` -> Host Port: `80`
        *   Container Port: `443` -> Host Port: `443`
        *   Container Port: `8080` -> Host Port: `8080`
    *   **Paths**:
        *   `/mnt/user/appdata/charon/data` -> `/app/data` (or `/mnt/user/appdata/cpmp/data` -> `/app/data` for backward compatibility)
        *   `/mnt/user/appdata/charon/caddy_data` -> `/data` (or `/mnt/user/appdata/cpmp/caddy_data` -> `/data` for backward compatibility)
        *   `/mnt/user/appdata/charon/caddy_config` -> `/config` (or `/mnt/user/appdata/cpmp/caddy_config` -> `/config` for backward compatibility)
3.  **Apply**: Click Done to pull and start.

## Troubleshooting

### App can't reach Caddy

**Symptom**: "Caddy unreachable" errors in logs

**Solution**: Since both run in the same container, this usually means Caddy failed to start. Check logs:
```bash
docker-compose logs app
```

### Certificates not working

**Symptom**: HTTP works but HTTPS fails

**Check**:
1. Port 80/443 are accessible from the internet
2. DNS points to your server
3. Caddy logs: `docker-compose logs app | grep -i acme`

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
image: ghcr.io/wikid82/charon:v1.0.0

docker-compose up -d
```

## Building from Source

```bash
# Build multi-arch images
docker buildx build --platform linux/amd64,linux/arm64 -t charon:local .

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

## Performance Tuning

For high-traffic deployments:

```yaml
# docker-compose.yml
services:
  app:
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
