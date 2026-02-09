# DNS Provider Types

This document describes the DNS provider types available in Charon for DNS-01 challenge validation during SSL certificate issuance.

## Overview

Charon supports multiple DNS provider types to accommodate different deployment scenarios:

| Provider Type | Use Case | Security Level |
|---------------|----------|----------------|
| **API-Based** | Cloudflare, Route53, DigitalOcean, etc. | ✅ Recommended |
| **RFC 2136** | Self-hosted BIND9, PowerDNS, Knot DNS | ✅ Recommended |
| **Webhook** | Custom DNS APIs, automation platforms | ⚠️ Moderate |
| **Script** | Legacy tools, custom integrations | ⚠️ High Risk |

---

## RFC 2136 (Dynamic DNS)

RFC 2136 Dynamic DNS Update allows Charon to directly update DNS records on authoritative DNS servers that support the protocol, using TSIG authentication for security.

### Use Cases

- Self-hosted BIND9, PowerDNS, or Knot DNS servers
- Enterprise environments with existing DNS infrastructure
- Air-gapped networks without external API access
- ISP or hosting provider managed DNS with RFC 2136 support

### Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `nameserver` | ✅ | — | DNS server hostname or IP address |
| `tsig_key_name` | ✅ | — | TSIG key name (e.g., `acme-update.`) |
| `tsig_key_secret` | ✅ | — | Base64-encoded TSIG key secret |
| `port` | ❌ | `53` | DNS server port |
| `tsig_algorithm` | ❌ | `hmac-sha256` | TSIG algorithm (see below) |
| `zone` | ❌ | — | DNS zone override (auto-detected if not set) |

### TSIG Algorithms

| Algorithm | Recommendation |
|-----------|----------------|
| `hmac-sha256` | ✅ **Recommended** — Good balance of security and compatibility |
| `hmac-sha384` | ✅ Secure — Higher security, wider key |
| `hmac-sha512` | ✅ Secure — Maximum security |
| `hmac-sha1` | ⚠️ Legacy — Use only if required by older systems |
| `hmac-md5` | ❌ **Deprecated** — Avoid; cryptographically weak |

### Example Configuration

```json
{
  "type": "rfc2136",
  "nameserver": "ns1.example.com",
  "port": 53,
  "tsig_key_name": "acme-update.",
  "tsig_key_secret": "base64EncodedSecretKey==",
  "tsig_algorithm": "hmac-sha256",
  "zone": "example.com"
}
```

### Generating a TSIG Key (BIND9)

```bash
# Generate a new TSIG key
tsig-keygen -a hmac-sha256 acme-update > /etc/bind/acme-update.key

# Contents of generated key file:
# key "acme-update" {
#     algorithm hmac-sha256;
#     secret "base64EncodedSecretKey==";
# };
```

### Security Notes

- **Network Security**: Ensure the DNS server is reachable from Charon (firewall rules, VPN)
- **Key Permissions**: TSIG keys should have minimal permissions (only `_acme-challenge` records)
- **Key Rotation**: Rotate TSIG keys periodically (recommended: every 90 days)
- **TLS Not Supported**: RFC 2136 uses UDP/TCP without encryption; use network-level security

---

## Webhook Provider

The Webhook provider enables integration with custom DNS APIs or automation platforms by sending HTTP requests to user-defined endpoints.

### Use Cases

- Custom internal DNS management APIs
- Integration with automation platforms (Ansible AWX, Rundeck, etc.)
- DNS providers without native Charon support
- Multi-system orchestration workflows

### Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `create_url` | ✅ | — | URL to call when creating TXT records |
| `delete_url` | ✅ | — | URL to call when deleting TXT records |
| `auth_header` | ❌ | — | HTTP header name for authentication (e.g., `Authorization`) |
| `auth_value` | ❌ | — | HTTP header value (e.g., `Bearer token123`) |
| `timeout_seconds` | ❌ | `30` | Request timeout in seconds |
| `retry_count` | ❌ | `3` | Number of retry attempts on failure |
| `insecure_skip_verify` | ❌ | `false` | Skip TLS verification (⚠️ dev only) |

### URL Template Variables

The following variables are available in `create_url` and `delete_url`:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{fqdn}}` | Full record name | `_acme-challenge.example.com` |
| `{{domain}}` | Base domain | `example.com` |
| `{{value}}` | TXT record value | `dGVzdC12YWx1ZQ==` |
| `{{ttl}}` | Record TTL | `120` |

### Example Configuration

```json
{
  "type": "webhook",
  "create_url": "https://dns-api.example.com/records?action=create&fqdn={{fqdn}}&value={{value}}",
  "delete_url": "https://dns-api.example.com/records?action=delete&fqdn={{fqdn}}",
  "auth_header": "Authorization",
  "auth_value": "Bearer your-api-token",
  "timeout_seconds": 30,
  "retry_count": 3
}
```

### Webhook Request Format

**Create Request:**

```http
POST {{create_url}}
Content-Type: application/json
{{auth_header}}: {{auth_value}}

{
  "fqdn": "_acme-challenge.example.com",
  "domain": "example.com",
  "value": "challenge-token-value",
  "ttl": 120
}
```

**Delete Request:**

```http
POST {{delete_url}}
Content-Type: application/json
{{auth_header}}: {{auth_value}}

{
  "fqdn": "_acme-challenge.example.com",
  "domain": "example.com"
}
```

### Expected Responses

| Status Code | Meaning |
|-------------|---------|
| `200`, `201`, `204` | Success |
| `4xx` | Client error — check configuration |
| `5xx` | Server error — will retry based on `retry_count` |

### Security Notes

- **HTTPS Required**: Non-localhost URLs must use HTTPS
- **Authentication**: Always use `auth_header` and `auth_value` for production
- **Timeouts**: Set appropriate timeouts to avoid blocking certificate issuance
- **`insecure_skip_verify`**: Never enable in production; only for local development with self-signed certs

---

## Script Provider

The Script provider executes shell scripts to manage DNS records, enabling integration with legacy systems or tools without API access.

### ⚠️ HIGH-RISK PROVIDER

> **Warning**: Scripts execute with container privileges. Only use when no other option is available. Thoroughly audit all scripts before deployment.

### Use Cases

- Legacy DNS management tools (nsupdate wrappers, custom CLIs)
- Systems requiring SSH-based updates
- Complex multi-step DNS workflows
- Air-gapped environments with local tooling

### Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `script_path` | ✅ | — | Path to script (must be in `/scripts/`) |
| `timeout_seconds` | ❌ | `60` | Maximum script execution time |
| `env_vars` | ❌ | — | Environment variables (`KEY=VALUE` format) |

### Script Interface

Scripts receive DNS operation details via environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `DNS_ACTION` | Operation type | `create` or `delete` |
| `DNS_FQDN` | Full record name | `_acme-challenge.example.com` |
| `DNS_DOMAIN` | Base domain | `example.com` |
| `DNS_VALUE` | TXT record value (create only) | `challenge-token` |
| `DNS_TTL` | Record TTL (create only) | `120` |

**Exit Codes:**

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Failure (generic) |
| `2` | Configuration error |

### Example Configuration

```json
{
  "type": "script",
  "script_path": "/scripts/dns-update.sh",
  "timeout_seconds": 60,
  "env_vars": "DNS_SERVER=ns1.example.com,SSH_KEY_PATH=/secrets/dns-key"
}
```

### Example Script

```bash
#!/bin/bash
# /scripts/dns-update.sh
set -euo pipefail

case "$DNS_ACTION" in
  create)
    echo "Creating TXT record: $DNS_FQDN = $DNS_VALUE"
    nsupdate -k /etc/bind/keys/update.key <<EOF
server $DNS_SERVER
update add $DNS_FQDN $DNS_TTL TXT "$DNS_VALUE"
send
EOF
    ;;
  delete)
    echo "Deleting TXT record: $DNS_FQDN"
    nsupdate -k /etc/bind/keys/update.key <<EOF
server $DNS_SERVER
update delete $DNS_FQDN TXT
send
EOF
    ;;
  *)
    echo "Unknown action: $DNS_ACTION" >&2
    exit 1
    ;;
esac
```

### Security Requirements

| Requirement | Details |
|-------------|---------|
| **Script Location** | Must be in `/scripts/` directory (enforced) |
| **Permissions** | Script must be executable (`chmod +x`) |
| **Audit** | Review all scripts before deployment |
| **Secrets** | Use mounted secrets, never hardcode credentials |
| **Timeouts** | Set appropriate timeouts to prevent hanging |

### Security Notes

- **Container Privileges**: Scripts run with full container privileges
- **Path Restriction**: Scripts must reside in `/scripts/` to prevent arbitrary execution
- **No User Input**: Script path cannot contain user-supplied data
- **Logging**: All script executions are logged to audit trail
- **Resource Limits**: Use `timeout_seconds` to prevent runaway scripts
- **Testing**: Test scripts thoroughly in non-production before deployment

---

## Provider Comparison

| Feature | RFC 2136 | Webhook | Script |
|---------|----------|---------|--------|
| **Setup Complexity** | Medium | Low | High |
| **Security** | High (TSIG) | Medium (HTTPS) | Low (shell) |
| **Flexibility** | DNS servers only | HTTP APIs | Unlimited |
| **Debugging** | DNS tools | HTTP logs | Script logs |
| **Recommended For** | Self-hosted DNS | Custom APIs | Legacy only |

## Related Documentation

- [DNS Provider Auto-Detection](./dns-auto-detection.md) — Automatic provider identification
- [Multi-Credential DNS Support](./multi-credential.md) — Managing multiple credentials per provider
- [Key Rotation](./key-rotation.md) — Credential rotation best practices
- [Audit Logging](./audit-logging.md) — Tracking DNS operations

---

_Last Updated: January 2026_
_Version: 1.4.0_
