# Fix CrowdSec Persistence & Offline Status

## Goal Description
The CrowdSec Security Engine is reported as "Offline" on the dashboard. This is caused by the lack of data persistence in the Docker container.
The `docker-entrypoint.sh` and `Dockerfile` currently configure CrowdSec to use ephemeral paths (`/etc/crowdsec` and `/var/lib/crowdsec/data`) which are not linked to the persistent volume `/app/data/crowdsec`.
Consequently, every container restart generates a new Machine ID and loses enrollment credentials, causing the dashboard to see the old instance as offline.

## User Review Required
> [!IMPORTANT]
> **Re-Enrollment Required**: After this fix is applied, the user will need to re-enroll their instance once. The new identity will persist across future restarts.
> **Mode Configuration**: The user must ensure `CERBERUS_SECURITY_CROWDSEC_MODE` is set to `local` in their environment or `docker-compose.yml`.

## Proposed Changes

### Docker & Scripts
#### [MODIFY] [docker-entrypoint.sh](file:///projects/Charon/docker-entrypoint.sh)
- Update CrowdSec initialization logic to map runtime directories to persistence:
  - Check for `/app/data/crowdsec/config` and `/app/data/crowdsec/data`.
  - If missing, populate from `/etc/crowdsec` (defaults).
  - Use symbolic links or environment variables (`DATA`) to point to `/app/data/crowdsec/...`.
  - Ensure `cscli` commands operate on the persistent configuration.

#### [MODIFY] [docker-compose.yml](file:///projects/Charon/docker-compose.yml)
- Update comments to explicitly recommend setting `CERBERUS_SECURITY_CROWDSEC_MODE=local` to avoid confusion.

## Verification Plan

### Manual Verification
1. **Persistence Test**:
   - Deploy the updated container.
   - Enter container: `docker exec -it charon sh`.
   - Run `cscli machines list` and note the Machine ID.
   - Modify a file in `/etc/crowdsec` (e.g., `touch /etc/crowdsec/test_persist`).
   - Restart container: `docker restart charon`.
   - Enter container again.
   - Verify `cscli machines list` shows the **SAME** Machine ID.
   - Verify `/etc/crowdsec/test_persist` still exists.

2. **Online Enrollment Test**:
   - Enroll the instance: `cscli console enroll <enroll-key>`.
   - Restart container.
   - Check `cscli console status` (if available) or verify on Dashboard that it remains "Online".

### Automated Tests
- None (requires Docker runtime test, which is manual in this context).
