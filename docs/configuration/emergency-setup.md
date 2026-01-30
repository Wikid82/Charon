# Emergency Break Glass Protocol - Configuration Guide

**Version:** 1.0
**Last Updated:** January 26, 2026
**Purpose:** Complete reference for configuring emergency break glass access

---

## Table of Contents

- [Overview](#overview)
- [Environment Variables Reference](#environment-variables-reference)
- [Docker Compose Examples](#docker-compose-examples)
- [Firewall Configuration](#firewall-configuration)
- [Secrets Manager Integration](#secrets-manager-integration)
- [Security Hardening](#security-hardening)

---

## Overview

Charon's emergency break glass protocol provides a 3-tier system for emergency access recovery:

- **Tier 1:** Emergency token via main application endpoint (Layer 7 bypass)
- **Tier 2:** Separate emergency server on dedicated port (network isolation)
- **Tier 3:** Direct system access (SSH/console)

This guide covers configuration for Tiers 1 and 2. Tier 3 requires only SSH access to the host.

---

## Environment Variables Reference

### Required Variables

#### `CHARON_EMERGENCY_TOKEN`

**Purpose:** Secret token for emergency break glass access (Tier 1 & 2)
**Format:** 64-character hexadecimal string
**Security:** CRITICAL - Store in secrets manager, never commit to version control

**Generation:**

```bash
# Recommended method (OpenSSL)
openssl rand -hex 32

# Alternative (Python)
python3 -c "import secrets; print(secrets.token_hex(32))"

# Alternative (/dev/urandom)
head -c 32 /dev/urandom | xxd -p -c 64
```

**Example:**

```yaml
environment:
  - CHARON_EMERGENCY_TOKEN=a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2
```

**Validation:**

- Minimum length: 32 characters (produces 64-char hex)
- Must be hexadecimal (0-9, a-f)
- Must be unique per deployment
- Rotate every 90 days

---

### Optional Variables

#### `CHARON_MANAGEMENT_CIDRS`

**Purpose:** IP ranges allowed to use emergency token (Tier 1)
**Format:** Comma-separated CIDR notation
**Default:** `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8` (RFC1918 + localhost)

**Examples:**

```yaml
# Office network only
- CHARON_MANAGEMENT_CIDRS=192.168.1.0/24

# Office + VPN
- CHARON_MANAGEMENT_CIDRS=192.168.1.0/24,10.8.0.0/24

# Multiple offices
- CHARON_MANAGEMENT_CIDRS=192.168.1.0/24,192.168.2.0/24,10.10.0.0/16

# Allow from anywhere (NOT RECOMMENDED)
- CHARON_MANAGEMENT_CIDRS=0.0.0.0/0,::/0
```

**Security Notes:**

- Be as restrictive as possible
- Never use `0.0.0.0/0` in production
- Include VPN subnet if using VPN for emergency access
- Update when office networks change

#### `CHARON_EMERGENCY_SERVER_ENABLED`

**Purpose:** Enable separate emergency server on dedicated port (Tier 2)
**Format:** Boolean (`true` or `false`)
**Default:** `false`

**When to enable:**

- ✅ Production deployments with CrowdSec
- ✅ High-security environments
- ✅ Deployments with restrictive firewalls
- ❌ Simple home labs (Tier 1 sufficient)

**Example:**

```yaml
environment:
  - CHARON_EMERGENCY_SERVER_ENABLED=true
```

#### `CHARON_EMERGENCY_BIND`

**Purpose:** Address and port for emergency server (Tier 2)
**Format:** `IP:PORT`
**Default:** `127.0.0.1:2020`
**Note:** Port 2020 avoids conflict with Caddy admin API (port 2019)

**Options:**

```yaml
# Localhost only (most secure - requires SSH tunnel)
- CHARON_EMERGENCY_BIND=127.0.0.1:2020

# Listen on all interfaces (DANGER - requires firewall rules)
- CHARON_EMERGENCY_BIND=0.0.0.0:2020

# Specific internal IP (VPN interface)
- CHARON_EMERGENCY_BIND=10.8.0.1:2020

# IPv6 localhost
- CHARON_EMERGENCY_BIND=[::1]:2020

# Dual-stack all interfaces
- CHARON_EMERGENCY_BIND=0.0.0.0:2020  # or [::]:2020 for IPv6
```

**⚠️ Security Warning:** Never bind to `0.0.0.0` without firewall protection. Use SSH tunneling instead.

#### `CHARON_EMERGENCY_USERNAME`

**Purpose:** Basic Auth username for emergency server (Tier 2)
**Format:** String
**Default:** None (Basic Auth disabled)

**Example:**

```yaml
environment:
  - CHARON_EMERGENCY_USERNAME=admin
```

**Security Notes:**

- Optional but recommended
- Use strong, unique username (not "admin" in production)
- Combine with strong password
- Consider using mTLS instead (future enhancement)

#### `CHARON_EMERGENCY_PASSWORD`

**Purpose:** Basic Auth password for emergency server (Tier 2)
**Format:** String
**Default:** None (Basic Auth disabled)

**Example:**

```yaml
environment:
  - CHARON_EMERGENCY_PASSWORD=${EMERGENCY_PASSWORD}  # From .env file
```

**Security Notes:**

- NEVER hardcode in docker-compose.yml
- Use `.env` file or secrets manager
- Minimum 20 characters recommended
- Rotate every 90 days

---

## Docker Compose Examples

### Example 1: Minimal Configuration (Homelab)

**Use case:** Simple home lab, Tier 1 only, no emergency server

```yaml
version: '3.8'

services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
      - "8080:8080"
    volumes:
      - charon_data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - TZ=UTC
      - CHARON_ENV=production
      - CHARON_ENCRYPTION_KEY=${CHARON_ENCRYPTION_KEY}  # From .env
      - CHARON_EMERGENCY_TOKEN=${CHARON_EMERGENCY_TOKEN}  # From .env

volumes:
  charon_data:
    driver: local
```

**.env file:**

```bash
# Generate with: openssl rand -base64 32
CHARON_ENCRYPTION_KEY=your-32-byte-base64-key-here

# Generate with: openssl rand -hex 32
CHARON_EMERGENCY_TOKEN=your-64-char-hex-token-here
```

---

### Example 2: Production Configuration (Tier 1 + Tier 2)

**Use case:** Production deployment with emergency server, VPN access

```yaml
version: '3.8'

services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
      - "8080:8080"
      # Emergency server (localhost only - use SSH tunnel)
      - "127.0.0.1:2020:2020"
    volumes:
      - charon_data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - TZ=UTC
      - CHARON_ENV=production
      - CHARON_ENCRYPTION_KEY=${CHARON_ENCRYPTION_KEY}

      # Emergency Token (Tier 1)
      - CHARON_EMERGENCY_TOKEN=${CHARON_EMERGENCY_TOKEN}
      - CHARON_MANAGEMENT_CIDRS=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16

      # Emergency Server (Tier 2)
      - CHARON_EMERGENCY_SERVER_ENABLED=true
      - CHARON_EMERGENCY_BIND=0.0.0.0:2020
      - CHARON_EMERGENCY_USERNAME=${CHARON_EMERGENCY_USERNAME}
      - CHARON_EMERGENCY_PASSWORD=${CHARON_EMERGENCY_PASSWORD}
    healthcheck:
      test: ["CMD", "curl", "--fail", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  charon_data:
    driver: local
```

**.env file:**

```bash
CHARON_ENCRYPTION_KEY=your-32-byte-base64-key-here
CHARON_EMERGENCY_TOKEN=your-64-char-hex-token-here
CHARON_EMERGENCY_USERNAME=emergency-admin
CHARON_EMERGENCY_PASSWORD=your-strong-password-here
```

---

### Example 3: Security-Hardened Configuration

**Use case:** High-security environment with Docker secrets, read-only filesystem

```yaml
version: '3.8'

services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped
    read_only: true
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    security_opt:
      - no-new-privileges:true
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
      - "8080:8080"
      - "127.0.0.1:2020:2020"
    volumes:
      - charon_data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
      # tmpfs for writable directories
      - type: tmpfs
        target: /tmp
        tmpfs:
          size: 100M
      - type: tmpfs
        target: /var/log/caddy
        tmpfs:
          size: 100M
    secrets:
      - charon_encryption_key
      - charon_emergency_token
      - charon_emergency_password
    environment:
      - TZ=UTC
      - CHARON_ENV=production
      - CHARON_ENCRYPTION_KEY_FILE=/run/secrets/charon_encryption_key
      - CHARON_EMERGENCY_TOKEN_FILE=/run/secrets/charon_emergency_token
      - CHARON_MANAGEMENT_CIDRS=10.8.0.0/24  # VPN subnet only
      - CHARON_EMERGENCY_SERVER_ENABLED=true
      - CHARON_EMERGENCY_BIND=0.0.0.0:2020
      - CHARON_EMERGENCY_USERNAME=emergency-admin
      - CHARON_EMERGENCY_PASSWORD_FILE=/run/secrets/charon_emergency_password

volumes:
  charon_data:
    driver: local

secrets:
  charon_encryption_key:
    external: true
  charon_emergency_token:
    external: true
  charon_emergency_password:
    external: true
```

**Create secrets:**

```bash
# Create secrets from files
echo "your-encryption-key" | docker secret create charon_encryption_key -
echo "your-emergency-token" | docker secret create charon_emergency_token -
echo "your-emergency-password" | docker secret create charon_emergency_password -

# Verify secrets
docker secret ls
```

---

### Example 4: Development Configuration

**Use case:** Local development, emergency server for testing

```yaml
version: '3.8'

services:
  charon:
    image: ghcr.io/wikid82/charon:nightly
    container_name: charon-dev
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "8080:8080"
      - "2020:2020"  # Emergency server on all interfaces for testing
    volumes:
      - charon_data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - TZ=UTC
      - CHARON_ENV=development
      - CHARON_DEBUG=1
      - CHARON_ENCRYPTION_KEY=dev-key-not-for-production-32bytes
      - CHARON_EMERGENCY_TOKEN=test-emergency-token-for-e2e-32chars
      - CHARON_EMERGENCY_SERVER_ENABLED=true
      - CHARON_EMERGENCY_BIND=0.0.0.0:2020
      - CHARON_EMERGENCY_USERNAME=admin
      - CHARON_EMERGENCY_PASSWORD=admin

volumes:
  charon_data:
    driver: local
```

**⚠️ WARNING:** This configuration is ONLY for local development. Never use in production.

---

## Firewall Configuration

### iptables Rules (Linux)

**Block public access to emergency port:**

```bash
# Allow localhost
iptables -A INPUT -i lo -p tcp --dport 2020 -j ACCEPT

# Allow VPN subnet (example: 10.8.0.0/24)
iptables -A INPUT -s 10.8.0.0/24 -p tcp --dport 2020 -j ACCEPT

# Block everything else
iptables -A INPUT -p tcp --dport 2020 -j DROP

# Save rules
iptables-save > /etc/iptables/rules.v4
```

### UFW Rules (Ubuntu/Debian)

```bash
# Allow from specific subnet only
ufw allow from 10.8.0.0/24 to any port 2020 proto tcp

# Enable firewall
ufw enable

# Verify rules
ufw status numbered
```

### firewalld Rules (RHEL/CentOS)

```bash
# Create new zone for emergency access
firewall-cmd --permanent --new-zone=emergency
firewall-cmd --permanent --zone=emergency --add-source=10.8.0.0/24
firewall-cmd --permanent --zone=emergency --add-port=2020/tcp

# Reload firewall
firewall-cmd --reload

# Verify
firewall-cmd --zone=emergency --list-all
```

### Docker Network Isolation

**Create dedicated network for emergency access:**

```yaml
services:
  charon:
    networks:
      - public
      - emergency

networks:
  public:
    driver: bridge
  emergency:
    driver: bridge
    internal: true  # No external connectivity
```

---

## Secrets Manager Integration

### HashiCorp Vault

**Store secrets:**

```bash
# Store emergency token
vault kv put secret/charon/emergency \
  token="$(openssl rand -hex 32)" \
  username="emergency-admin" \
  password="$(openssl rand -base64 32)"

# Read secrets
vault kv get secret/charon/emergency
```

**Docker Compose with Vault:**

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    environment:
      - CHARON_EMERGENCY_TOKEN=${VAULT_CHARON_EMERGENCY_TOKEN}
      - CHARON_EMERGENCY_USERNAME=${VAULT_CHARON_EMERGENCY_USERNAME}
      - CHARON_EMERGENCY_PASSWORD=${VAULT_CHARON_EMERGENCY_PASSWORD}
```

**Retrieve from Vault:**

```bash
# Export secrets from Vault
export VAULT_CHARON_EMERGENCY_TOKEN=$(vault kv get -field=token secret/charon/emergency)
export VAULT_CHARON_EMERGENCY_USERNAME=$(vault kv get -field=username secret/charon/emergency)
export VAULT_CHARON_EMERGENCY_PASSWORD=$(vault kv get -field=password secret/charon/emergency)

# Start with secrets
docker-compose up -d
```

### AWS Secrets Manager

**Store secrets:**

```bash
# Create secret
aws secretsmanager create-secret \
  --name charon/emergency \
  --description "Charon emergency break glass credentials" \
  --secret-string '{
    "token": "YOUR_TOKEN_HERE",
    "username": "emergency-admin",
    "password": "YOUR_PASSWORD_HERE"
  }'
```

**Retrieve in Docker Compose:**

```bash
#!/bin/bash

# Retrieve secret
SECRET=$(aws secretsmanager get-secret-value \
  --secret-id charon/emergency \
  --query SecretString \
  --output text)

# Parse JSON and export
export CHARON_EMERGENCY_TOKEN=$(echo $SECRET | jq -r '.token')
export CHARON_EMERGENCY_USERNAME=$(echo $SECRET | jq -r '.username')
export CHARON_EMERGENCY_PASSWORD=$(echo $SECRET | jq -r '.password')

# Start Charon
docker-compose up -d
```

### Azure Key Vault

**Store secrets:**

```bash
# Create Key Vault
az keyvault create \
  --name charon-vault \
  --resource-group charon-rg \
  --location eastus

# Store secrets
az keyvault secret set \
  --vault-name charon-vault \
  --name emergency-token \
  --value "YOUR_TOKEN_HERE"

az keyvault secret set \
  --vault-name charon-vault \
  --name emergency-username \
  --value "emergency-admin"

az keyvault secret set \
  --vault-name charon-vault \
  --name emergency-password \
  --value "YOUR_PASSWORD_HERE"
```

**Retrieve secrets:**

```bash
#!/bin/bash

# Retrieve secrets
export CHARON_EMERGENCY_TOKEN=$(az keyvault secret show \
  --vault-name charon-vault \
  --name emergency-token \
  --query value -o tsv)

export CHARON_EMERGENCY_USERNAME=$(az keyvault secret show \
  --vault-name charon-vault \
  --name emergency-username \
  --query value -o tsv)

export CHARON_EMERGENCY_PASSWORD=$(az keyvault secret show \
  --vault-name charon-vault \
  --name emergency-password \
  --query value -o tsv)

# Start Charon
docker-compose up -d
```

---

## Security Hardening

### Best Practices Checklist

- [ ] **Emergency token** stored in secrets manager (not in docker-compose.yml)
- [ ] **Token rotation** scheduled every 90 days
- [ ] **Management CIDRs** restricted to minimum necessary networks
- [ ] **Emergency server** bound to localhost only (127.0.0.1)
- [ ] **SSH tunneling** used for emergency server access
- [ ] **Firewall rules** block public access to port 2019
- [ ] **Basic Auth** enabled on emergency server with strong credentials
- [ ] **Audit logging** monitored for emergency access
- [ ] **Alerts configured** for emergency token usage
- [ ] **Backup procedures** tested and documented
- [ ] **Recovery runbooks** reviewed by team
- [ ] **Quarterly drills** scheduled to test procedures

### Network Hardening

**VPN-Only Access:**

```yaml
environment:
  # Only allow emergency access from VPN subnet
  - CHARON_MANAGEMENT_CIDRS=10.8.0.0/24

  # Emergency server listens on VPN interface only
  - CHARON_EMERGENCY_BIND=10.8.0.1:2020
```

**mTLS for Emergency Server** (Future Enhancement):

```yaml
environment:
  - CHARON_EMERGENCY_TLS_ENABLED=true
  - CHARON_EMERGENCY_TLS_CERT=/run/secrets/emergency_tls_cert
  - CHARON_EMERGENCY_TLS_KEY=/run/secrets/emergency_tls_key
  - CHARON_EMERGENCY_TLS_CA=/run/secrets/emergency_tls_ca
```

### Monitoring & Alerting

**Prometheus Metrics:**

```yaml
# Emergency access metrics
charon_emergency_token_attempts_total{result="success"}
charon_emergency_token_attempts_total{result="failure"}
charon_emergency_server_requests_total
```

**Alert Rules:**

```yaml
groups:
  - name: charon_emergency_access
    rules:
      - alert: EmergencyTokenUsed
        expr: increase(charon_emergency_token_attempts_total{result="success"}[5m]) > 0
        labels:
          severity: critical
        annotations:
          summary: "Emergency break glass token was used"
          description: "Someone used the emergency token to disable security. Review audit logs."

      - alert: EmergencyTokenBruteForce
        expr: increase(charon_emergency_token_attempts_total{result="failure"}[5m]) > 10
        labels:
          severity: warning
        annotations:
          summary: "Multiple failed emergency token attempts detected"
          description: "Possible brute force attack on emergency endpoint."
```

---

## Validation & Testing

### Configuration Validation

```bash
# Validate docker-compose.yml syntax
docker-compose config

# Verify environment variables are set
docker-compose config | grep EMERGENCY

# Test container starts successfully
docker-compose up -d
docker logs charon | grep -i emergency
```

### Functional Testing

**Test Tier 1:**

```bash
# Test emergency token works
curl -X POST https://charon.example.com/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: $CHARON_EMERGENCY_TOKEN"

# Expected: {"success":true, ...}
```

**Test Tier 2:**

```bash
# Create SSH tunnel
ssh -L 2020:localhost:2020 admin@server &

# Test emergency server health
curl http://localhost:2020/health

# Test emergency endpoint
curl -X POST http://localhost:2020/emergency/security-reset \
  -H "X-Emergency-Token: $CHARON_EMERGENCY_TOKEN" \
  -u admin:password

# Close tunnel
kill %1
```

---

## Related Documentation

- [Emergency Lockout Recovery Runbook](../runbooks/emergency-lockout-recovery.md)
- [Emergency Token Rotation](../runbooks/emergency-token-rotation.md)
- [Security Documentation](../security.md)
- [Break Glass Protocol Design](../plans/break_glass_protocol_redesign.md)

---

**Version History:**

- v1.0 (2026-01-26): Initial release
