# DNS Challenge Troubleshooting

This guide covers common issues with DNS-01 ACME challenges and how to resolve them.

## Table of Contents

- [Connection Test Failures](#connection-test-failures)
- [Certificate Issuance Failures](#certificate-issuance-failures)
- [DNS Propagation Issues](#dns-propagation-issues)
- [Provider-Specific Errors](#provider-specific-errors)
- [Network and Firewall Issues](#network-and-firewall-issues)
- [Credential Problems](#credential-problems)
- [Debugging Tips](#debugging-tips)

## Connection Test Failures

### Invalid Credentials

**Symptoms:**

- "Invalid API token" or "Unauthorized" error
- Connection test fails immediately

**Solutions:**

1. Verify credentials were copied correctly (no extra spaces/newlines)
2. Check token/key hasn't expired
3. Ensure token has required permissions:
   - Cloudflare: Zone → DNS → Edit
   - AWS: `route53:ChangeResourceRecordSets`
   - DigitalOcean: Write scope
4. Regenerate credentials if necessary
5. Update configuration in Charon with new credentials

### DNS Provider Unreachable

**Symptoms:**

- "Connection timeout" or "Network error"
- Test hangs for 30+ seconds

**Solutions:**

1. Check internet connectivity from Charon server
2. Verify firewall allows outbound HTTPS (port 443)
3. Test DNS resolution:

   ```bash
   # Test DNS provider API endpoint resolution
   nslookup api.cloudflare.com
   curl -I https://api.cloudflare.com
   ```

4. Check provider status page for service outages
5. Verify proxy settings if using HTTP proxy

### Zone/Domain Not Found

**Symptoms:**

- "Hosted zone not found"
- "Domain not configured"

**Solutions:**

1. Verify domain is added to DNS provider account
2. Ensure domain status is Active (not Pending)
3. Check nameservers are correctly configured:

   ```bash
   dig NS example.com +short
   ```

4. Wait 24-48 hours if nameservers were recently changed
5. Verify API token is scoped to include the domain (if applicable)

## Certificate Issuance Failures

### DNS Propagation Timeout

**Symptoms:**

- Certificate issuance fails after 2-5 minutes
- Error: "DNS propagation timeout" or "TXT record not found"

**Solutions:**

1. **Increase propagation timeout:**
   - Navigate to DNS Provider settings
   - Increase Propagation Timeout to 180-300 seconds
   - Save and retry certificate issuance

2. **Verify DNS propagation:**

   ```bash
   # Check if TXT record was created
   dig _acme-challenge.example.com TXT +short

   # Check from multiple DNS servers
   dig _acme-challenge.example.com TXT @8.8.8.8 +short
   dig _acme-challenge.example.com TXT @1.1.1.1 +short
   ```

3. **Check DNS provider configuration:**
   - Ensure domain's nameservers point to your DNS provider
   - Verify no conflicting DNS records exist
   - Check DNSSEC is properly configured (if enabled)

4. **Provider-specific adjustments:**
   - **Cloudflare:** Usually fast (60s), check Cloudflare status
   - **Route 53:** Often slow (120-180s), increase timeout
   - **DigitalOcean:** Moderate (90s), verify nameservers

### ACME Server Errors

**Symptoms:**

- "Too many requests" or "Rate limit exceeded"
- "Invalid response from ACME server"

**Solutions:**

1. **Let's Encrypt rate limits:**
   - 50 certificates per domain per week
   - 5 failed validation attempts per hour
   - Wait before retrying if limit hit
   - Use staging environment for testing:

     ```bash
     # In Caddy config (for testing only)
     acme_ca https://acme-staging-v02.api.letsencrypt.org/directory
     ```

2. **ACME challenge failures:**
   - Review Charon logs for specific ACME error codes
   - Verify TXT record was created correctly
   - Ensure DNS provider has write permissions
   - Test with a different DNS provider (if available)

3. **Boulder (Let's Encrypt) validation errors:**
   - Error indicates which authoritative DNS server was queried
   - Verify all nameservers return the TXT record
   - Check for split-horizon DNS issues

### Wildcard Domain Issues

**Symptoms:**

- Wildcard certificate issuance fails
- Error: "DNS challenge required for wildcard domains"

**Solutions:**

1. Verify DNS provider is configured in Charon
2. Select DNS provider when creating proxy host
3. Ensure wildcard syntax is correct: `*.example.com`
4. Confirm DNS provider has permissions for the root domain
5. Test with non-wildcard domain first (e.g., `test.example.com`)

## DNS Propagation Issues

### Slow Global Propagation

**Symptoms:**

- Certificate issuance succeeds locally but fails remotely
- Inconsistent results from different DNS resolvers

**Diagnostic Commands:**

```bash
# Check propagation from multiple locations
dig _acme-challenge.example.com TXT @8.8.8.8
dig _acme-challenge.example.com TXT @1.1.1.1
dig _acme-challenge.example.com TXT @208.67.222.222

# Check TTL of existing records
dig example.com +noall +answer | grep -i ttl
```

**Solutions:**

1. Increase Propagation Timeout to 300-600 seconds
2. Lower TTL on existing DNS records (set 1 hour before changes)
3. Wait for previous high-TTL records to expire
4. Use DNS provider with faster global propagation (e.g., Cloudflare)

### Cached DNS Records

**Symptoms:**

- Old TXT records still visible after deletion
- Certificate renewal fails with "Incorrect TXT record"

**Solutions:**

1. Wait for TTL expiry (default: 300-3600 seconds)
2. Flush local DNS cache:

   ```bash
   # Linux
   sudo systemd-resolve --flush-caches

   # macOS
   sudo dscacheutil -flushcache
   ```

3. Test with authoritative nameservers directly:

   ```bash
   dig _acme-challenge.example.com TXT @ns1.your-provider.com
   ```

## Provider-Specific Errors

### Cloudflare

**Error:** `Cloudflare API error 6003: Invalid request headers`

- **Cause:** Malformed API token
- **Solution:** Regenerate token, ensure no invisible characters

**Error:** `Cloudflare API error 10000: Authentication error`

- **Cause:** Token revoked or expired
- **Solution:** Create new token with correct permissions

**Error:** `Zone is not active`

- **Cause:** Nameservers not updated
- **Solution:** Update domain nameservers, wait for activation

### AWS Route 53

**Error:** `AccessDenied: User is not authorized`

- **Cause:** IAM permissions insufficient
- **Solution:** Attach IAM policy with `route53:ChangeResourceRecordSets`

**Error:** `InvalidChangeBatch: RRSet with duplicate name`

- **Cause:** Conflicting TXT record already exists
- **Solution:** Remove manual `_acme-challenge` TXT records

**Error:** `Throttling: Rate exceeded`

- **Cause:** Too many API requests
- **Solution:** Increase polling interval to 15-20 seconds

### DigitalOcean

**Error:** `The resource you requested could not be found`

- **Cause:** Domain not in DigitalOcean DNS
- **Solution:** Add domain to Networking → Domains

**Error:** `Unable to authenticate you`

- **Cause:** Token has Read scope instead of Write
- **Solution:** Regenerate token with Write scope

## Network and Firewall Issues

### Outbound HTTPS Blocked

**Symptoms:**

- Connection tests timeout
- "Network unreachable" errors

**Diagnostic Commands:**

```bash
# Test connectivity to DNS provider API
curl -v https://api.cloudflare.com/client/v4/user
curl -v https://api.digitalocean.com/v2/account

# Check if firewall is blocking
sudo iptables -L OUTPUT -v -n | grep -i drop
```

**Solutions:**

1. Allow outbound HTTPS (port 443) in firewall
2. Whitelist DNS provider API endpoints
3. Configure HTTP proxy if required:

   ```bash
   export HTTP_PROXY=http://proxy.example.com:8080
   export HTTPS_PROXY=http://proxy.example.com:8080
   ```

### DNS Resolution Failures

**Symptoms:**

- Cannot resolve DNS provider API domains
- Error: "No such host"

**Diagnostic Commands:**

```bash
# Test DNS resolution
nslookup api.cloudflare.com
dig api.cloudflare.com

# Check /etc/resolv.conf
cat /etc/resolv.conf
```

**Solutions:**

1. Verify DNS server is configured correctly
2. Test with public DNS (8.8.8.8, 1.1.1.1)
3. Check network interface configuration
4. Restart networking service

## Credential Problems

### Encryption Key Issues

**Symptoms:**

- "Encryption key not configured"
- "Failed to decrypt credentials"

**Solutions:**

1. **Set encryption key:**

   ```bash
   # Generate new key
   openssl rand -base64 32

   # Set environment variable
   export CHARON_ENCRYPTION_KEY="your-base64-key-here"
   ```

2. **Verify key in environment:**

   ```bash
   echo $CHARON_ENCRYPTION_KEY
   # Should show 44-character base64 string
   ```

3. **Docker/Docker Compose:**

   ```yaml
   # docker-compose.yml
   services:
     charon:
       environment:
         - CHARON_ENCRYPTION_KEY=${CHARON_ENCRYPTION_KEY}
   ```

4. **Restart Charon after setting key**

### Credentials Lost After Restart

**Symptoms:**

- DNS provider shows "Unconfigured" status after restart
- Connection test fails with "Invalid credentials"

**Cause:** Encryption key changed or missing

**Solutions:**

1. Ensure `CHARON_ENCRYPTION_KEY` is persistent (not temporary)
2. Add to systemd service file, docker-compose, or .env file
3. Never change encryption key (all credentials will be unrecoverable)
4. If key is lost, reconfigure all DNS providers

## Debugging Tips

### Enable Debug Logging

```bash
# Set log level in Charon configuration
export CHARON_LOG_LEVEL=debug

# Restart Charon
```

### Review Charon Logs

```bash
# Docker
docker logs charon -f --tail 100

# Systemd
journalctl -u charon -f -n 100

# Look for lines containing:
# - "DNS provider"
# - "ACME challenge"
# - "Certificate issuance"
```

### Test DNS Provider Manually

Use Caddy directly to test DNS provider:

```bash
# Create test Caddyfile
cat > Caddyfile << 'EOF'
test.example.com {
    tls {
        dns cloudflare {env.CLOUDFLARE_API_TOKEN}
    }
    respond "Test successful"
}
EOF

# Run Caddy with test config
CLOUDFLARE_API_TOKEN=your-token caddy run --config Caddyfile
```

### Check ACME Challenge TXT Record

Monitor DNS changes during certificate issuance:

```bash
# Watch for TXT record creation
watch -n 5 'dig _acme-challenge.example.com TXT +short'

# Check authoritative nameservers
dig _acme-challenge.example.com TXT @$(dig NS example.com +short | head -1)
```

### Common Log Messages

**Success:**

```
[INFO] DNS provider test successful
[INFO] ACME challenge completed
[INFO] Certificate issued successfully
```

**Errors:**

```
[ERROR] Failed to create TXT record: <reason>
[ERROR] DNS propagation timeout after 120 seconds
[ERROR] ACME validation failed: <acme-error>
```

## Getting Help

If you're still experiencing issues:

1. **Review Documentation:**
   - [DNS Providers Overview](../guides/dns-providers.md)
   - Provider-specific setup guides
   - [Security best practices](../security/best-practices.md)

2. **Gather Information:**
   - Charon version and log excerpt
   - DNS provider type
   - Error message (exact text)
   - Network environment (Docker, VPS, etc.)

3. **Check Known Issues:**
   - [GitHub Issues](https://github.com/Wikid82/Charon/issues)
   - Release notes and changelogs

4. **Contact Support:**
   - Open a GitHub issue with debug logs
   - Join community Discord/forum
   - Include relevant diagnostic output (sanitize credentials)

## Related Documentation

- [DNS Providers Guide](../guides/dns-providers.md)
- [Cloudflare Setup](../guides/dns-providers/cloudflare.md)
- [AWS Route 53 Setup](../guides/dns-providers/route53.md)
- [DigitalOcean Setup](../guides/dns-providers/digitalocean.md)
- [Certificate Management](../guides/certificates.md)
