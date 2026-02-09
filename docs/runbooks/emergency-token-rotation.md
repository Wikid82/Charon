# Emergency Token Rotation Runbook

**Version:** 1.0
**Last Updated:** January 26, 2026
**Purpose:** Secure procedure for rotating the emergency break glass token

---

## When to Rotate

Rotate the emergency token in these situations:

- **Scheduled rotation** — Every 90 days (recommended)
- **After use** — Token was used during an incident
- **Suspected compromise** — Token may have been exposed in logs, screenshots, or shared insecurely
- **Personnel changes** — Team member with token access has left
- **Security audit** — Compliance requirement or security policy mandate

---

## Prerequisites

- ✅ Access to Charon configuration (`docker-compose.yml` or secrets manager)
- ✅ Ability to restart Charon container
- ✅ Write access to secrets management system (if used)
- ✅ Documentation of where token is stored

---

## Step-by-Step Rotation Procedure

### Step 1: Generate New Token

Generate a cryptographically secure 64-character hex token:

```bash
# Using OpenSSL (recommended)
openssl rand -hex 32

# Example output
a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0e1f2

# Using /dev/urandom
head -c 32 /dev/urandom | xxd -p -c 64

# Using Python
python3 -c "import secrets; print(secrets.token_hex(32))"
```

**Requirements:**

- Minimum 32 characters (produces 64-char hex)
- Use cryptographically secure random generator
- Never reuse old tokens

### Step 2: Document Token Securely

Store the new token in your secrets management system:

**HashiCorp Vault:**

```bash
vault kv put secret/charon/emergency-token \
  token='NEW_TOKEN_HERE'
```

**AWS Secrets Manager:**

```bash
aws secretsmanager update-secret \
  --secret-id charon/emergency-token \
  --secret-string 'NEW_TOKEN_HERE'
```

**Azure Key Vault:**

```bash
az keyvault secret set \
  --vault-name charon-vault \
  --name emergency-token \
  --value 'NEW_TOKEN_HERE'
```

**Password Manager:**

- Store in "Charon Emergency Access" entry
- Add expiration reminder (90 days)
- Share with authorized personnel only

### Step 3: Update Docker Compose Configuration

#### Option A: Environment Variable (Less Secure)

Edit `docker-compose.yml`:

```yaml
services:
  charon:
    environment:
      - CHARON_EMERGENCY_TOKEN=NEW_TOKEN_HERE  # <-- Update this
```

#### Option B: Docker Secrets (More Secure)

```yaml
services:
  charon:
    secrets:
      - charon_emergency_token
    environment:
      - CHARON_EMERGENCY_TOKEN_FILE=/run/secrets/charon_emergency_token

secrets:
  charon_emergency_token:
    external: true
```

Create Docker secret:

```bash
echo "NEW_TOKEN_HERE" | docker secret create charon_emergency_token -
```

#### Option C: Environment File (Recommended)

Create `.env` file (add to `.gitignore`):

```bash
# .env
CHARON_EMERGENCY_TOKEN=NEW_TOKEN_HERE
```

Update `docker-compose.yml`:

```yaml
services:
  charon:
    env_file:
      - .env
```

### Step 4: Restart Charon Container

```bash
# Using docker-compose
docker-compose down
docker-compose up -d

# Or restart existing container
docker-compose restart charon

# Verify container started successfully
docker logs charon --tail 20
```

**Expected log output:**

```text
[INFO] Emergency token configured (64 characters)
[INFO] Emergency bypass middleware enabled
[INFO] Management CIDRs: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
```

### Step 5: Verify New Token Works

Test the new token from an allowed IP:

```bash
# Test emergency reset endpoint
curl -X POST https://charon.example.com/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: NEW_TOKEN_HERE" \
  -H "Content-Type: application/json"

# Expected response
{
  "success": true,
  "message": "All security modules have been disabled",
  "disabled_modules": [...]
}
```

**If testing in production:** Re-enable security immediately:

```bash
# Navigate to Cerberus Dashboard
# Toggle security modules back ON
```

### Step 6: Verify Old Token is Revoked

Test that the old token no longer works:

```bash
# Test with old token (should fail)
curl -X POST https://charon.example.com/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: OLD_TOKEN_HERE" \
  -H "Content-Type: application/json"

# Expected response (401 Unauthorized)
{
  "error": "Invalid emergency token",
  "code": 401
}
```

### Step 7: Update Documentation

Update all locations where the token is documented:

- [ ] Password manager entry
- [ ] Secrets management system
- [ ] Runbooks (if token is referenced)
- [ ] Team wiki or internal docs
- [ ] Incident response procedures
- [ ] Backup/recovery documentation

### Step 8: Notify Team

Inform authorized personnel:

```markdown
Subject: [ACTION REQUIRED] Charon Emergency Token Rotated

The Charon emergency break glass token has been rotated as part of our regular security maintenance.

**Action Required:**
- Update your local password manager with the new token
- Retrieve new token from: [secrets management location]
- Old token is no longer valid as of: [timestamp]

**Next Rotation:** [90 days from now]

If you need access to the new token, contact: [security team contact]
```

---

## Emergency Rotation (Compromise Suspected)

If the token has been compromised, follow this expedited procedure:

### Immediate Actions (within 1 hour)

1. **Rotate token immediately** (Steps 1-5 above)
2. **Review audit logs** for unauthorized emergency access:

```bash
# Check for emergency token usage
docker logs charon | grep -i "emergency"

# Check audit logs
curl http://localhost:8080/api/v1/audit-logs | jq '.[] | select(.action | contains("emergency"))'
```

1. **Alert security team** if unauthorized access detected
2. **Disable compromised accounts** that may have used the token

### Investigation (within 24 hours)

1. **Determine exposure scope:**
   - Was token in logs or screenshots?
   - Was token shared via insecure channel (email, Slack)?
   - Who had access to the token?
   - Was token committed to version control?

2. **Check for signs of abuse:**
   - Review recent configuration changes
   - Check for new proxy hosts or certificates
   - Verify ACL rules haven't been modified
   - Review CrowdSec decision history

3. **Document incident:**
   - Create incident report
   - Timeline of exposure
   - Impact assessment
   - Remediation actions taken

### Remediation

1. **Revoke access** for compromised accounts
2. **Rotate all related secrets** (database passwords, API keys)
3. **Implement additional controls:**
   - Require 2FA for emergency access (future enhancement)
   - Implement emergency token session limits
   - Add approval workflow for emergency access
4. **Update policies** to prevent future exposure

---

## Automation Script

Save this script as `rotate-emergency-token.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Charon Emergency Token Rotation Script${NC}"
echo "========================================"
echo ""

# Step 1: Generate new token
echo -e "${YELLOW}Step 1: Generating new token...${NC}"
NEW_TOKEN=$(openssl rand -hex 32)
echo "New token generated: ${NEW_TOKEN:0:16}...${NEW_TOKEN: -16}"
echo ""

# Step 2: Backup old configuration
echo -e "${YELLOW}Step 2: Backing up current configuration...${NC}"
BACKUP_FILE="docker-compose.yml.backup-$(date +%Y%m%d-%H%M%S)"
cp docker-compose.yml "$BACKUP_FILE"
echo "Backup saved to: $BACKUP_FILE"
echo ""

# Step 3: Update docker-compose.yml
echo -e "${YELLOW}Step 3: Updating docker-compose.yml...${NC}"
sed -i.bak "s/CHARON_EMERGENCY_TOKEN=.*/CHARON_EMERGENCY_TOKEN=${NEW_TOKEN}/" docker-compose.yml
echo "Configuration updated"
echo ""

# Step 4: Restart container
echo -e "${YELLOW}Step 4: Restarting Charon container...${NC}"
docker-compose restart charon
sleep 5
echo "Container restarted"
echo ""

# Step 5: Verify new token
echo -e "${YELLOW}Step 5: Verifying new token...${NC}"
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: ${NEW_TOKEN}" \
  -H "Content-Type: application/json")

if echo "$RESPONSE" | grep -q '"success":true'; then
  echo -e "${GREEN}✓ New token verified successfully${NC}"
else
  echo -e "${RED}✗ Token verification failed${NC}"
  echo "Response: $RESPONSE"
  exit 1
fi
echo ""

# Step 6: Save token securely
echo -e "${YELLOW}Step 6: Token rotation complete${NC}"
echo ""
echo "========================================"
echo -e "${GREEN}NEXT STEPS:${NC}"
echo "1. Save new token to password manager:"
echo "   ${NEW_TOKEN}"
echo ""
echo "2. Update secrets manager (Vault, AWS, Azure)"
echo "3. Notify team of token rotation"
echo "4. Test old token is revoked"
echo "5. Schedule next rotation: $(date -d '+90 days' +%Y-%m-%d)"
echo "========================================"
```

Make executable:

```bash
chmod +x rotate-emergency-token.sh
```

Run:

```bash
./rotate-emergency-token.sh
```

---

## Compliance Checklist

For organizations with compliance requirements:

- [ ] Token rotation documented in change log
- [ ] Rotation approved by security team
- [ ] Old token marked as revoked in secrets manager
- [ ] Access to new token limited to authorized personnel
- [ ] Token rotation logged in audit trail
- [ ] Backup configuration saved securely
- [ ] Team notification sent and acknowledged
- [ ] Next rotation scheduled (90 days)

---

## Troubleshooting

### Error: New Token Not Working After Rotation

**Symptom:** New token returns 401 Unauthorized.

**Causes:**

1. Token not saved correctly in configuration
2. Container not restarted after update
3. Token contains whitespace or line breaks
4. Environment variable not exported

**Solution:**

```bash
# Verify environment variable
docker exec charon env | grep CHARON_EMERGENCY_TOKEN

# Check logs for token loading
docker logs charon | grep -i "emergency token"

# Restart container
docker-compose restart charon
```

### Error: Container Won't Start After Update

**Symptom:** Container exits immediately after restart.

**Cause:** Malformed docker-compose.yml or invalid token format.

**Solution:**

```bash
# Validate docker-compose.yml syntax
docker-compose config

# Restore backup
cp docker-compose.yml.backup docker-compose.yml

# Fix syntax error
vim docker-compose.yml

# Restart
docker-compose up -d
```

### Error: Lost Access to Old Token

**Symptom:** Need to verify old token is revoked, but don't have it.

**Solution:**

```bash
# Check backup configuration
grep CHARON_EMERGENCY_TOKEN docker-compose.yml.backup-*

# Or check container environment (if not restarted)
docker exec charon env | grep CHARON_EMERGENCY_TOKEN
```

---

## Security Best Practices

1. **Never commit tokens to version control**
   - Add to `.gitignore`: `.env`, `docker-compose.override.yml`
   - Use pre-commit hooks to scan for secrets
   - Use `git-secrets` or `trufflehog`

2. **Use secrets management systems**
   - HashiCorp Vault
   - AWS Secrets Manager
   - Azure Key Vault
   - Kubernetes Secrets (with encryption at rest)

3. **Limit token access**
   - Only senior engineers and ops team
   - Require 2FA for secrets manager access
   - Audit who accesses the token

4. **Rotate regularly**
   - Every 90 days (at minimum)
   - After any security incident
   - When team members leave

5. **Monitor emergency token usage**
   - Set up alerts for emergency access
   - Review audit logs weekly
   - Investigate any unexpected usage

6. **Test recovery procedures**
   - Quarterly disaster recovery drills
   - Verify backup token storage works
   - Ensure team knows how to retrieve token

---

## Related Documentation

- [Emergency Lockout Recovery Runbook](./emergency-lockout-recovery.md)
- [Security Documentation](../security.md)
- [Configuration Guide](../configuration/emergency-setup.md)

---

**Version History:**

- v1.0 (2026-01-26): Initial release
