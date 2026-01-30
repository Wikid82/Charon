# Emergency Lockout Recovery Runbook

**Version:** 1.0
**Last Updated:** January 26, 2026
**Status:** Production Ready
**Severity:** 🔴 CRITICAL

---

## Purpose

This runbook provides step-by-step procedures to regain access to Charon when security modules
(ACL, WAF, CrowdSec, Rate Limiting) have blocked legitimate administrative access.

**When to use this:** You see "403 Forbidden", "Blocked by access control list", or cannot access
the Charon web interface.

---

## Symptoms: How to Recognize a Lockout

### Symptom 1: ACL Lockout

```text
HTTP 403 Forbidden
{"error": "Blocked by access control list"}
```

**Cause:** Your IP address is not in the ACL whitelist, or is in a blacklist.

### Symptom 2: WAF Block

```text
HTTP 403 Forbidden
{"error": "Request blocked by Web Application Firewall"}
```

**Cause:** Your request triggered a WAF rule (e.g., suspicious pattern in URL or headers).

### Symptom 3: CrowdSec Ban

```text
HTTP 403 Forbidden
{"error": "Your IP has been banned"}
```

**Cause:** CrowdSec flagged your IP as malicious (brute force, scanning, etc.).

### Symptom 4: Rate Limiting

```text
HTTP 429 Too Many Requests
{"error": "Rate limit exceeded"}
```

**Cause:** Too many requests from your IP in a short time period.

---

## Test Environment Configuration

### Rate Limiting in Test Environments

For test and development environments (`CHARON_ENV=test|e2e|development`), the emergency rate limiter is set to **50 attempts per minute** to facilitate testing and debugging.

**Production environments** maintain strict rate limiting: **5 attempts per 5 minutes**.

⚠️ **Security Warning:** Always set `CHARON_ENV=production` (or omit the variable) in production deployments to enforce proper rate limiting.

### Testing Both Tiers

E2E tests validate both break glass tiers to ensure defense in depth:

**Tier 1 (Main Endpoint):**
```bash
curl -X POST http://localhost:8080/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: $TOKEN"
```

**Tier 2 (Emergency Server):**
```bash
curl -X POST http://localhost:2020/emergency/security-reset \
  -H "X-Emergency-Token: $TOKEN" \
  -u admin:password
```

**Environment Variable Reference:**

| Environment | Max Attempts | Window | Use Case |
|-------------|--------------|--------|----------|
| `production` (default) | 5 | 5 minutes | Production deployments |
| `test` | 50 | 1 minute | Unit/integration tests |
| `e2e` | 50 | 1 minute | E2E test suites |
| `development` | 50 | 1 minute | Local development |

---

## Recovery Tiers

Charon provides a **3-Tier Break Glass Protocol**. Start with Tier 1 and escalate if needed.

| Tier | Method | Use When | Prerequisites |
| ---- | ------ | -------- | ------------- |
| **Tier 1** | Emergency Token (Digital Key) | Application accessible | Emergency token, management network access |
| **Tier 2** | Emergency Server (Sidecar Door) | Caddy/CrowdSec blocking | SSH access, emergency server enabled |
| **Tier 3** | Direct System Access (Physical Key) | Complete failure | SSH/console access to host |

---

## Tier 1: Digital Key (Emergency Token)

**Use when:** The Charon application is reachable, but security middleware is blocking you.

### Prerequisites

- ✅ Emergency token value (64-char hex string from `CHARON_EMERGENCY_TOKEN`)
- ✅ HTTPS connection to Charon (HTTP also works for local development)
- ✅ Source IP in management network (default: RFC1918 private IPs)

### Step-by-Step Procedure

#### Step 1: Retrieve Emergency Token

The emergency token is configured via the `CHARON_EMERGENCY_TOKEN` environment variable:

```bash
# If using docker-compose.yml
grep CHARON_EMERGENCY_TOKEN docker-compose.yml

# If using .env file
grep CHARON_EMERGENCY_TOKEN .env

# From running container
docker exec charon env | grep CHARON_EMERGENCY_TOKEN

# From secrets manager (example: AWS)
aws secretsmanager get-secret-value --secret-id charon/emergency-token
```

**Security Note:** Store this token in a password manager or secrets management system.

#### Step 2: Send Emergency Reset Request

```bash
# Basic usage
curl -X POST https://charon.example.com/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: your-64-char-hex-token-here" \
  -H "Content-Type: application/json"
```

**Expected Response (Success):**

```json
{
  "success": true,
  "message": "All security modules have been disabled",
  "disabled_modules": [
    "feature.cerberus.enabled",
    "security.acl.enabled",
    "security.waf.enabled",
    "security.rate_limit.enabled",
    "security.crowdsec.enabled"
  ],
  "timestamp": "2026-01-26T10:30:45Z"
}
```

#### Step 3: Wait for Settings Propagation

Security settings update immediately, but allow 5 seconds for full propagation:

```bash
sleep 5
```

#### Step 4: Verify Access Restored

```bash
# Test health endpoint
curl https://charon.example.com/api/v1/health

# Expected response
{"status": "ok", "version": "1.0.0"}
```

#### Step 5: Access Web Interface

Open your browser and navigate to:

```text
https://charon.example.com:8080
```

You should now have full access to the Charon management interface.

### Troubleshooting Tier 1

#### Error: 403 Forbidden (before reset)

**Symptom:** Emergency reset endpoint returns 403 before you can submit the token.

**Cause:** Tier 1 is blocked at the Caddy/CrowdSec layer (Layer 7 reverse proxy).

**Solution:** Proceed to [Tier 2: Emergency Server](#tier-2-sidecar-door-emergency-server).

#### Error: 401 Unauthorized

**Symptom:** Emergency reset returns 401 with message "Invalid emergency token".

**Cause:** Token mismatch - the token you provided doesn't match `CHARON_EMERGENCY_TOKEN`.

**Solution:**

1. Verify token value from configuration
2. Check for extra whitespace or line breaks
3. Ensure token is at least 32 characters long
4. Regenerate token if necessary (see [Token Rotation Guide](./emergency-token-rotation.md))

#### Error: 429 Too Many Requests

**Symptom:** Emergency reset returns 429 with message "Rate limit exceeded".

**Cause:** Too many failed emergency token attempts (5 per minute per IP).

**Solution:**

1. Wait 60 seconds for rate limit to reset
2. Verify token value before retrying
3. Use Tier 2 if you cannot wait

#### Error: 501 Not Implemented

**Symptom:** Emergency reset returns 501 with message "Emergency token not configured".

**Cause:** `CHARON_EMERGENCY_TOKEN` environment variable is not set.

**Solution:**

1. Use [Tier 2: Emergency Server](#tier-2-sidecar-door-emergency-server)
2. Or use [Tier 3: Direct System Access](#tier-3-physical-key-direct-system-access) to set the token

#### Error: Source IP Not in Management Network

**Symptom:** 403 with message "Emergency access denied: IP not in management network".

**Cause:** Your IP is not in the allowed management CIDRs (default: RFC1918 private IPs).

**Solution:**

1. Connect via VPN to access management network
2. Use SSH tunnel from allowed IP (see Tier 2)
3. Update `CHARON_MANAGEMENT_CIDRS` to include your IP (requires Tier 3 access)

---

## Tier 2: Sidecar Door (Emergency Server)

**Use when:** Tier 1 is blocked at the Caddy/CrowdSec layer, or you need a separate entry point.

### Prerequisites

- ✅ VPN or SSH access to Docker host
- ✅ Emergency server enabled (`CHARON_EMERGENCY_SERVER_ENABLED=true`)
- ✅ Knowledge of emergency server port (default: 2019)
- ✅ Basic Auth credentials (if configured)

### Architecture Diagram

```text
[Public Traffic:443]           [SSH Tunnel:2019]
    ↓                               ↓
[Caddy Reverse Proxy]          [Emergency Server]
    ↓ (WAF, ACL, CrowdSec)         ↓ (Minimal Security)
[Main Application:8080]        [Emergency Handlers]
    ↓                               ↓
[BLOCKED]                      [DIRECT ACCESS ✅]
```

### Step-by-Step Procedure

#### Step 1: SSH to Docker Host

```bash
# SSH to server
ssh admin@docker-host.example.com
```

#### Step 2: Verify Emergency Server is Running

```bash
# Check container environment
docker exec charon env | grep EMERGENCY

# Expected output
CHARON_EMERGENCY_SERVER_ENABLED=true
CHARON_EMERGENCY_BIND=127.0.0.1:2019
CHARON_EMERGENCY_USERNAME=admin
CHARON_EMERGENCY_PASSWORD=<password>
```

#### Step 3: Create SSH Tunnel

**From your local machine**, create a tunnel to the emergency port:

```bash
# Open tunnel (port 2019 on localhost → port 2019 on server)
ssh -L 2019:localhost:2019 admin@docker-host.example.com

# Keep this terminal open - tunnel stays active
```

#### Step 4: Test Emergency Server Health

**From your local machine** (in a new terminal):

```bash
# Health check
curl http://localhost:2019/health

# Expected response
{"status":"ok","server":"emergency"}
```

#### Step 5: Send Emergency Reset Request

```bash
# With Basic Auth
curl -X POST http://localhost:2019/emergency/security-reset \
  -H "X-Emergency-Token: your-64-char-hex-token-here" \
  -u admin:your-emergency-password

# Without Basic Auth (if not configured)
curl -X POST http://localhost:2019/emergency/security-reset \
  -H "X-Emergency-Token: your-64-char-hex-token-here"
```

**Expected Response:**

```json
{
  "success": true,
  "message": "All security modules have been disabled",
  "disabled_modules": [...]
}
```

#### Step 6: Verify Access Restored

```bash
# Test main application
curl https://charon.example.com/api/v1/health
```

#### Step 7: Close SSH Tunnel

```bash
# In the terminal with the open tunnel, press Ctrl+C
# Or use the kill command
kill $SSH_TUNNEL_PID
```

### Troubleshooting Tier 2

#### Error: Connection Refused (Port 2019)

**Cause:** Emergency server is not enabled or not running.

**Verification:**

```bash
# Check if emergency server is enabled
docker exec charon env | grep CHARON_EMERGENCY_SERVER_ENABLED

# Check if port is listening
docker exec charon netstat -tlnp | grep 2019
```

**Solution:**

1. Enable emergency server in `docker-compose.yml`:

```yaml
environment:
  - CHARON_EMERGENCY_SERVER_ENABLED=true
  - CHARON_EMERGENCY_BIND=127.0.0.1:2019
```

1. Restart container:

```bash
docker-compose restart charon
```

#### Error: 401 Unauthorized (Basic Auth)

**Cause:** Basic Auth credentials are incorrect.

**Solution:**

1. Verify credentials from configuration:

```bash
docker exec charon env | grep CHARON_EMERGENCY_
```

1. Reset password in `docker-compose.yml` if needed

#### Error: SSH Tunnel Fails

**Cause:** Firewall blocking SSH port 22, or SSH service not running.

**Solution:**

1. Verify SSH service is running:

```bash
systemctl status sshd
```

1. Check firewall rules allow SSH:

```bash
sudo ufw status | grep 22
```

1. Use alternative port if 22 is blocked:

```bash
ssh -p 2222 -L 2019:localhost:2019 admin@server
```

---

## Tier 3: Physical Key (Direct System Access)

**Use when:** All application-level recovery methods have failed, or you need to perform system-level repairs.

### Prerequisites

- ✅ Root or sudo access to Docker host
- ✅ Knowledge of container name (default: `charon` or `charon-e2e`)
- ✅ Backup access credentials (in case database needs restoration)

### Recovery Methods

#### Method 1: Clear CrowdSec Bans

If you're blocked by CrowdSec:

```bash
# SSH to host
ssh admin@docker-host.example.com

# List all bans
docker exec charon cscli decisions list

# Delete specific ban
docker exec charon cscli decisions delete --ip YOUR_IP

# Delete ALL bans (use with caution)
docker exec charon cscli decisions delete --all

# Verify decisions are cleared
docker exec charon cscli decisions list
# Should show: No decisions found
```

#### Method 2: Direct Database Access

Disable security modules directly in the database:

```bash
# Access SQLite database
docker exec -it charon sqlite3 /app/data/charon.db

# Disable all security modules
sqlite> UPDATE settings SET value = 'false' WHERE key = 'feature.cerberus.enabled';
sqlite> UPDATE settings SET value = 'false' WHERE key = 'security.acl.enabled';
sqlite> UPDATE settings SET value = 'false' WHERE key = 'security.waf.enabled';
sqlite> UPDATE settings SET value = 'false' WHERE key = 'security.rate_limit.enabled';
sqlite> UPDATE settings SET value = 'false' WHERE key = 'security.crowdsec.enabled';

# Update SecurityConfig table
sqlite> UPDATE security_configs SET enabled = 0;

# Verify changes
sqlite> SELECT key, value FROM settings WHERE key LIKE 'security.%';

# Exit SQLite
sqlite> .quit
```

#### Method 3: Restart with Security Disabled

Temporarily disable all security features:

```bash
# Stop container
docker stop charon

# Add environment override to docker-compose.yml
# Or start with inline environment variable
docker start charon -e CERBERUS_DISABLED=true

# Alternative: Edit docker-compose.yml
vim docker-compose.yml
# Add: - CERBERUS_DISABLED=true

# Restart container
docker-compose up -d charon
```

#### Method 4: Kill Caddy to Bypass Reverse Proxy

If CrowdSec is blocking at Caddy layer:

```bash
# Stop Caddy process (temporary)
docker exec charon pkill caddy

# Warning: This breaks TLS termination
# Only use for emergency access, then restart:
docker restart charon
```

#### Method 5: Docker Volume Inspection

Inspect and modify data without running the container:

```bash
# Find Charon data volume
docker volume ls | grep charon

# Mount volume to temporary container
docker run --rm -it -v charon_data:/data alpine sh

# Navigate to database
cd /data

# Use SQLite (if installed in Alpine)
apk add sqlite
sqlite3 charon.db

# Or copy database out for external editing
exit
docker cp charon:/app/data/charon.db ~/charon-backup.db
```

### Catastrophic Recovery: Destroy and Recreate

> ⚠️ **WARNING**: Last resort only - you will lose all configuration data

#### Step 1: Backup Everything

```bash
# Backup database
docker exec charon tar czf /tmp/backup.tar.gz /app/data
docker cp charon:/tmp/backup.tar.gz ~/charon-backup-$(date +%Y%m%d-%H%M%S).tar.gz

# Record current configuration
docker inspect charon > ~/charon-inspect-$(date +%Y%m%d-%H%M%S).json
```

#### Step 2: Destroy Container and Volume

```bash
# Stop and remove container
docker stop charon
docker rm charon

# DANGER: Remove data volume (all configuration will be lost)
docker volume rm charon_data
```

#### Step 3: Recreate with Fresh Configuration

```bash
# Recreate container
docker-compose up -d charon

# Wait for initialization
sleep 10

# Access with default credentials (if auth is implemented)
curl http://localhost:8080/api/v1/health
```

#### Step 4: Restore from Backup (Optional)

```bash
# Stop container
docker stop charon

# Extract backup
tar xzf ~/charon-backup-YYYYMMDD-HHMMSS.tar.gz -C /tmp

# Copy database back
docker cp /tmp/app/data/charon.db charon:/app/data/charon.db

# Start container
docker start charon
```

### Troubleshooting Tier 3

#### Error: Permission Denied (SQLite)

**Cause:** Database file is owned by the container user, not root.

**Solution:**

```bash
# Use docker exec instead of direct file access
docker exec -it charon sh -c "sqlite3 /app/data/charon.db 'UPDATE settings SET value=\"false\" WHERE key=\"security.acl.enabled\"'"
```

#### Error: Container Won't Start After Database Changes

**Cause:** Database corruption or invalid schema.

**Solution:**

1. Check container logs:

```bash
docker logs charon --tail 50
```

1. Restore from automated backup:

```bash
# List backups
docker exec charon ls -la /app/data/backups/

# Restore latest backup
docker exec charon cp /app/data/backups/charon_backup_YYYYMMDD_030000.db /app/data/charon.db

# Restart container
docker restart charon
```

#### Error: Volume Not Found

**Cause:** Volume was deleted or never created.

**Solution:**

```bash
# Recreate volume
docker volume create charon_data

# Restart container with new volume
docker-compose up -d charon
```

---

## Post-Recovery Tasks

After regaining access, perform these tasks to prevent future lockouts:

### Task 1: Review Audit Logs

Analyze what caused the lockout:

```bash
# View recent security events
curl http://localhost:8080/api/v1/audit-logs | jq

# Filter for security events
docker exec charon grep -i "acl_deny\|waf_block\|crowdsec" /var/log/charon.log
```

**Look for:**

- Repeated blocks of your IP
- Triggered WAF rules
- CrowdSec ban reasons

### Task 2: Adjust ACL Rules

If ACL caused the lockout:

1. Navigate to **Cerberus → Access Lists**
2. Review ACL rules that blocked you
3. Add your IP to whitelist:
   - Create new ACL: "Admin Whitelist"
   - Type: IP Whitelist
   - IP Ranges: `YOUR_IP/32`
   - Assign to all critical hosts
4. Save configuration

### Task 3: Rotate Emergency Token (If Compromised)

If you suspect the emergency token was exposed:

1. Generate new token:

```bash
openssl rand -hex 32
```

1. Update configuration:

```bash
# Edit docker-compose.yml
vim docker-compose.yml
# Change CHARON_EMERGENCY_TOKEN value

# Restart container
docker-compose up -d charon
```

1. See [Emergency Token Rotation Guide](./emergency-token-rotation.md) for detailed steps

### Task 4: Document the Incident

Create incident report:

```markdown
# Security Lockout Incident Report

**Date:** YYYY-MM-DD HH:MM
**Severity:** Critical / High / Medium / Low
**Duration:** X minutes/hours

## Incident Summary
Brief description of what happened

## Root Cause
Why the lockout occurred

## Recovery Method Used
Which tier was used to recover

## Lessons Learned
What we learned from this incident

## Action Items
- [ ] Adjust ACL rules
- [ ] Update documentation
- [ ] Train team on recovery procedures
- [ ] Implement additional monitoring
```

### Task 5: Update Monitoring/Alerting

Set up alerts to prevent future lockouts:

1. Navigate to **Cerberus → Notification Settings**
2. Configure webhook or email notifications
3. Enable alerts for:
   - High rate of ACL denials
   - Admin IP blocks
   - Emergency token usage
4. Test notification delivery

### Task 6: Review Management Network Configuration

Ensure your management networks are properly configured:

```bash
# Check current CIDRS
docker exec charon env | grep CHARON_MANAGEMENT_CIDRS

# Update in docker-compose.yml
vim docker-compose.yml
```

Add your office/VPN subnets:

```yaml
environment:
  - CHARON_MANAGEMENT_CIDRS=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,YOUR_OFFICE_SUBNET
```

### Task 7: Test Recovery Procedures

Schedule quarterly drills to practice recovery:

```bash
# Test Tier 1
curl -X POST https://charon.example.com/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: $CHARON_EMERGENCY_TOKEN"

# Test Tier 2 (if enabled)
ssh -L 2019:localhost:2019 admin@server
curl http://localhost:2019/health

# Test Tier 3 (in staging environment)
docker exec charon cscli decisions list
```

---

## Quick Reference Card

### One-Page Emergency Cheat Sheet

```bash
# ---------- TIER 1: EMERGENCY TOKEN ----------
curl -X POST https://charon.example.com/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: $CHARON_EMERGENCY_TOKEN"

# ---------- TIER 2: EMERGENCY SERVER ----------
# 1. SSH tunnel
ssh -L 2019:localhost:2019 admin@server.example.com

# 2. Reset via emergency port
curl -X POST http://localhost:2019/emergency/security-reset \
  -H "X-Emergency-Token: $CHARON_EMERGENCY_TOKEN" \
  -u admin:password

# ---------- TIER 3: DIRECT ACCESS ----------
# SSH to host
ssh admin@docker-host.example.com

# Clear CrowdSec bans
docker exec charon cscli decisions delete --all

# Disable security via database
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE settings SET value='false' WHERE key LIKE 'security.%.enabled';"

# Restart container
docker restart charon

# ---------- VERIFICATION ----------
# Test health endpoint
curl http://localhost:8080/api/v1/health

# Check logs
docker logs charon --tail 50

# Verify security is disabled
curl http://localhost:8080/api/v1/settings | grep security
```

### Emergency Contacts

| Role | Contact | Purpose |
| ---- | ------- | ------- |
| Platform Team | `platform@example.com` | Infrastructure issues |
| Security Team | `security@example.com` | Security policy questions |
| On-Call Engineer | `oncall@example.com` | 24/7 emergency support |

### Critical Environment Variables

```bash
# Emergency access
CHARON_EMERGENCY_TOKEN=<64-char-hex>
CHARON_MANAGEMENT_CIDRS=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16

# Emergency server (Tier 2)
CHARON_EMERGENCY_SERVER_ENABLED=true
CHARON_EMERGENCY_BIND=127.0.0.1:2019
CHARON_EMERGENCY_USERNAME=admin
CHARON_EMERGENCY_PASSWORD=<password>
```

---

## Appendix A: Recovery Decision Tree

```text
START: Cannot access Charon web interface
  ↓
Can you reach https://charon.example.com?
  ├─ YES → Try Tier 1 (Emergency Token)
  │   ↓
  │   Success?
  │     ├─ YES → [END] Access restored
  │     └─ NO → Try Tier 2 (Emergency Server)
  │         ↓
  │         Success?
  │           ├─ YES → [END] Access restored
  │           └─ NO → Proceed to Tier 3
  │
  └─ NO → Network issue or container down
      ↓
      Check container status
        ├─ Container running → Proceed to Tier 3
        └─ Container down → Start container, then Tier 1
```

---

## Appendix B: Common Error Codes

| Code | Message | Cause | Solution |
| ---- | ------- | ----- | -------- |
| 403 | Blocked by access control list | ACL blocking IP | Use Tier 1 or adjust ACL |
| 403 | Request blocked by WAF | WAF rule triggered | Use Tier 1 or disable WAF |
| 403 | Your IP has been banned | CrowdSec ban | Use Tier 3 to clear bans |
| 401 | Invalid emergency token | Token mismatch | Verify token value |
| 429 | Rate limit exceeded | Too many attempts | Wait 60 seconds |
| 501 | Emergency token not configured | Token not set | Use Tier 3 to set token |
| 500 | Internal server error | Application error | Check logs, use Tier 3 |

---

## Appendix C: Testing Checklist

Use this checklist to validate recovery procedures:

**Tier 1 Testing:**

- [ ] Emergency token retrieved from secure storage
- [ ] Token works from allowed IP (RFC1918)
- [ ] Token blocked from public IP
- [ ] Rate limiting works (5 attempts per minute)
- [ ] Audit logs capture emergency access
- [ ] Settings disabled successfully

**Tier 2 Testing:**

- [ ] SSH tunnel established successfully
- [ ] Emergency server health endpoint responds
- [ ] Basic Auth works (if configured)
- [ ] Emergency reset works via tunnel
- [ ] Tunnel closes cleanly

**Tier 3 Testing:**

- [ ] CrowdSec decisions cleared
- [ ] Database modifications persist
- [ ] Container restarts successfully
- [ ] Backup and restore works
- [ ] Logs show expected behavior

---

**Related Documentation:**

- [Emergency Token Rotation](./emergency-token-rotation.md)
- [Break Glass Protocol Design](../plans/break_glass_protocol_redesign.md)
- [Security Documentation](../security.md)
- [Configuration Guide](../configuration/emergency-setup.md)

---

**Version History:**

- v1.0 (2026-01-26): Initial release
- Author: Charon Project Team
- Maintained by: Security & Operations Team
