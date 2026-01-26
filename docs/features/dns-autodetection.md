# DNS Provider Auto-Detection

**Status:** ✅ Production Ready
**Version:** 1.0.0
**Last Updated:** January 4, 2026

---

## Overview

DNS Provider Auto-Detection is an intelligent system that automatically identifies which DNS provider manages your domain's nameservers. When configuring wildcard SSL certificates in Charon, you no longer need to manually select your DNS provider—Charon detects it for you in less than a second.

**Who Benefits:**

- **Managed Service Providers (MSPs):** Managing multiple customer domains across different DNS providers
- **System Administrators:** Setting up wildcard certificates for multiple domains
- **DevOps Teams:** Automating certificate provisioning workflows
- **Small Businesses:** Simplifying SSL certificate setup without technical expertise

**Key Benefits:**

- ⚡ **Instant Detection:** Identifies your DNS provider in 100-200ms
- 🎯 **High Accuracy:** Supports 10+ major DNS providers with confidence scoring
- ⏱️ **Time Savings:** Reduces setup time from 5-10 minutes to under 30 seconds
- 🛡️ **Error Prevention:** Eliminates misconfiguration from incorrect provider selection
- 🔄 **Smart Caching:** Remembers detection results for 1 hour to improve performance

---

## How It Works

DNS auto-detection uses a simple but powerful process:

1. **Domain Entry:** You enter a wildcard domain like `*.example.com`
2. **Nameserver Lookup:** Charon queries DNS for your domain's nameservers
3. **Pattern Matching:** System matches nameservers against 10+ known provider patterns
4. **Confidence Scoring:** Calculates confidence level based on match strength
5. **Auto-Selection:** Automatically selects provider if confidence is high (≥80%)
6. **Manual Override:** You can always override the auto-detected provider

**Technical Details:**

- Uses standard DNS NS (nameserver) record lookups
- Matches nameserver hostnames against built-in pattern database
- Case-insensitive pattern matching for reliability
- Results cached in-memory for 1 hour
- DNS lookup timeout: 10 seconds maximum
- Typical detection time: 100-200ms
- Cache hit time: <1ms

**Example Detection Flow:**

```
User enters: *.example.com
↓
DNS Lookup: example.com NS records
↓
Returns: ns1.cloudflare.com, ns2.cloudflare.com
↓
Pattern Match: "cloudflare.com" → Cloudflare
↓
Confidence: High (100% match)
↓
Result: ✓ Cloudflare detected (High confidence)
```

---

## Supported DNS Providers

Charon has built-in detection for these major DNS providers:

| Provider | Detection Pattern | Example Nameserver |
|----------|------------------|-------------------|
| **Cloudflare** | `cloudflare.com` | ns1.cloudflare.com |
| **Amazon Route 53** | `awsdns` | ns-123.awsdns-45.com |
| **DigitalOcean** | `digitalocean.com` | ns1.digitalocean.com |
| **Google Cloud DNS** | `googledomains.com`, `ns-cloud` | ns-cloud-a1.googledomains.com |
| **Microsoft Azure DNS** | `azure-dns` | ns1-01.azure-dns.com |
| **Namecheap** | `registrar-servers.com` | dns1.registrar-servers.com |
| **GoDaddy** | `domaincontrol.com` | ns01.domaincontrol.com |
| **Hetzner** | `hetzner.com`, `hetzner.de` | ns1.hetzner.com |
| **Vultr** | `vultr.com` | ns1.vultr.com |
| **DNSimple** | `dnsimple.com` | ns1.dnsimple.com |

**Note:** If your DNS provider isn't automatically detected, you can still select it manually from the dropdown. Detection is a convenience feature, not a requirement.

---

## Using Auto-Detection

### In the Web UI

**Step-by-Step: Creating a Wildcard Proxy Host**

1. **Navigate to Proxy Hosts**
   - Click **Proxy Hosts** in the sidebar
   - Click **Add Proxy Host** button

2. **Enter Domain Information**
   - **Domain Name:** Enter your domain (e.g., `app.example.com`)
   - Check **Force SSL** checkbox
   - Check **Use Wildcard Domain** checkbox
   - Domain automatically changes to `*.example.com`

3. **Auto-Detection Triggers**
   - Detection starts automatically (500ms after typing stops)
   - Loading spinner appears: "Detecting DNS provider..."
   - Detection completes in 100-200ms

4. **Review Detection Result**

   **High Confidence Example:**

   ```
   ✓ Cloudflare detected (High confidence)

   Nameservers:
   • ns1.cloudflare.com
   • ns2.cloudflare.com

   [✓ Use Cloudflare]  [Select Manually]
   ```

   **Medium/Low Confidence Example:**

   ```
   ⚠ DigitalOcean detected (Medium confidence)

   Nameservers:
   • ns1.digitalocean.com

   We detected DigitalOcean, but confidence is medium.
   Please verify this is correct.

   [Use DigitalOcean]  [Select Manually]
   ```

   **No Detection Example:**

   ```
   ✗ DNS provider not detected

   Nameservers:
   • ns1.customdns.example.com
   • ns2.customdns.example.com

   Please select your DNS provider manually.

   [Select Manually]
   ```

5. **Choose Action**
   - **Use [Provider]:** Auto-selects detected provider (recommended for high confidence)
   - **Select Manually:** Opens provider dropdown for manual selection

6. **Complete Configuration**
   - Select DNS credentials (or add new ones)
   - Configure other proxy settings
   - Click **Save**

**Tips:**

- Detection works best with production domains already using their final nameservers
- If detection fails, check that your domain's DNS is properly configured
- Manual selection is always available as a fallback

---

### Via API

**Detect DNS Provider Endpoint**

Manually trigger DNS provider detection for any domain.

```bash
curl -X POST https://your-charon-instance/api/v1/dns-providers/detect \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com"
  }'
```

**Successful Detection Response:**

```json
{
  "domain": "example.com",
  "detected": true,
  "provider_type": "cloudflare",
  "nameservers": [
    "ns1.cloudflare.com",
    "ns2.cloudflare.com"
  ],
  "confidence": "high",
  "suggested_provider": {
    "uuid": "550e8400-e29b-41d4-a716-446655440000",
    "name": "My Cloudflare Account",
    "provider_type": "cloudflare",
    "is_default": true
  }
}
```

**Failed Detection Response:**

```json
{
  "domain": "example.com",
  "detected": false,
  "nameservers": [
    "ns1.custom-dns.example.com"
  ],
  "confidence": "none",
  "suggested_provider": null
}
```

**Error Response:**

```json
{
  "domain": "example.com",
  "detected": false,
  "nameservers": [],
  "confidence": "none",
  "error": "DNS lookup timeout: no nameservers found"
}
```

**Request Parameters:**

- `domain` (string, required): Domain to detect (with or without wildcard `*`)

**Response Fields:**

- `domain` (string): The base domain that was checked
- `detected` (boolean): Whether a provider was successfully identified
- `provider_type` (string): Type identifier for the detected provider
- `nameservers` (array): List of nameserver hostnames found
- `confidence` (string): Confidence level - `"high"`, `"medium"`, `"low"`, or `"none"`
- `suggested_provider` (object): Matching configured DNS provider (if any)
- `error` (string): Error message if detection failed

---

**Get Detection Patterns Endpoint**

Retrieve the current built-in nameserver pattern database.

```bash
curl https://your-charon-instance/api/v1/dns-providers/detection-patterns \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Response:**

```json
{
  "patterns": {
    "cloudflare.com": "cloudflare",
    "awsdns": "route53",
    "digitalocean.com": "digitalocean",
    "googledomains.com": "googleclouddns",
    "ns-cloud": "googleclouddns",
    "azure-dns": "azure",
    "registrar-servers.com": "namecheap",
    "domaincontrol.com": "godaddy",
    "hetzner.com": "hetzner",
    "hetzner.de": "hetzner",
    "vultr.com": "vultr",
    "dnsimple.com": "dnsimple"
  }
}
```

**Response Format:**

- `patterns` (object): Map of nameserver patterns to provider type identifiers
- Pattern keys are substring matches (case-insensitive)
- Provider type values match Charon's DNS provider types

---

## Use Cases

### 1. Quick Wildcard Certificate Setup

**Scenario:** MSP managing 50+ customer domains across multiple DNS providers

**Before Auto-Detection:**

- Manually research DNS provider for each customer domain
- Look up nameservers using external tools (`dig`, `nslookup`)
- Risk of selecting wrong provider → certificate issuance fails
- Time per domain: 5-10 minutes
- Total time for 50 domains: 4-8 hours

**With Auto-Detection:**

- Enter customer's wildcard domain
- Provider detected automatically in <200ms
- One-click to use detected provider
- Time per domain: 30 seconds
- Total time for 50 domains: 25 minutes

**Time Savings:** 90%+ reduction in configuration time

---

### 2. Multi-Customer Management

**Scenario:** Service provider managing customers using different DNS providers

**Customer Portfolio:**

- `*.customer1.com` → Cloudflare (High confidence)
- `*.customer2.com` → Route53 (High confidence)
- `*.customer3.com` → DigitalOcean (High confidence)
- `*.customer4.com` → Azure DNS (High confidence)
- `*.customer5.com` → Namecheap (Medium confidence - verify)

**Benefits:**

- No need to remember which customer uses which provider
- Automatic correct provider suggestion
- Confidence levels flag domains needing verification
- Standardized workflow for all customers

---

### 3. Mixed Environment

**Scenario:** Company with domains split across multiple DNS providers

**Infrastructure:**

- Production domains (`*.prod.company.com`) → Cloudflare
- Development domains (`*.dev.company.com`) → DigitalOcean
- Legacy domains (`*.legacy.company.com`) → Namecheap
- Marketing domains (`*.marketing.company.com`) → Google Cloud DNS

**Challenge:** Developers frequently set up new wildcard proxies and forget which DNS provider manages each environment.

**Solution:** Auto-detection eliminates guesswork:

- Developers enter domain
- Correct provider automatically detected
- Zero configuration errors
- Faster deployments

---

### 4. Automation and CI/CD Integration

**Scenario:** Automated certificate provisioning in deployment pipelines

**API Integration Example:**

```bash
#!/bin/bash
# Automated wildcard certificate setup

DOMAIN="*.newcustomer.com"

# Detect DNS provider
DETECTION=$(curl -s -X POST \
  https://charon.company.internal/api/v1/dns-providers/detect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"domain\": \"$DOMAIN\"}")

PROVIDER_TYPE=$(echo "$DETECTION" | jq -r '.provider_type')
CONFIDENCE=$(echo "$DETECTION" | jq -r '.confidence')

if [ "$CONFIDENCE" = "high" ]; then
  echo "✓ Detected $PROVIDER_TYPE with high confidence"
  # Proceed with automatic certificate issuance
  # ... create proxy host with detected provider ...
else
  echo "⚠ Low confidence detection, manual verification required"
  exit 1
fi
```

**Benefits:**

- Fully automated provisioning
- Self-documenting configuration
- Confidence checks prevent misconfiguration
- Error handling built-in

---

## Troubleshooting

### Detection Failed

**Symptom:** "Could not detect DNS provider" or "DNS provider not detected"

**Possible Causes:**

1. **Domain Not Using Production Nameservers**
   - Domain recently registered
   - DNS not yet pointed to production provider
   - Still using registrar's default nameservers

2. **DNS Provider Not in Built-in Database**
   - Using a custom or niche DNS provider
   - Using self-hosted DNS servers
   - Using a regional provider not in the pattern list

3. **DNS Lookup Timeout or Network Issue**
   - DNS server not responding
   - Network connectivity problem
   - Firewall blocking DNS queries

4. **Domain Doesn't Exist**
   - Typo in domain name
   - Domain not yet registered
   - Domain expired

**Solutions:**

**Check Domain's Nameservers:**

```bash
# Linux/Mac
dig NS example.com +short

# Windows
nslookup -type=NS example.com
```

Expected output:

```
ns1.cloudflare.com.
ns2.cloudflare.com.
```

**Verify Nameserver Propagation:**

```bash
# Check multiple DNS servers
dig @8.8.8.8 NS example.com +short
dig @1.1.1.1 NS example.com +short
```

**Wait for DNS Propagation:**

- Initial DNS setup: Up to 48 hours
- DNS changes: Up to 24 hours
- Check again after propagation completes

**Use Manual Provider Selection:**

- Click **Select Manually** button
- Choose provider from dropdown
- Detection is optional—manual selection always works

**Check Network Connectivity:**

```bash
# Test DNS connectivity
dig cloudflare.com +short
```

---

### Wrong Provider Detected

**Symptom:** Detection suggests incorrect or unexpected provider

**Possible Causes:**

1. **Domain Recently Migrated**
   - Domain moved from old provider to new provider
   - DNS caching showing old records
   - Propagation incomplete

2. **Cached Detection Result**
   - Previous detection cached for 1 hour
   - Domain's DNS changed since last detection
   - Cache showing outdated information

3. **Shared Nameserver Pattern**
   - Some hosting providers use shared nameserver pools
   - Pattern match may be ambiguous
   - Multiple providers with similar nameserver naming

**Solutions:**

**Verify Current Nameservers:**

```bash
dig NS example.com +short
```

Compare with detected nameservers in Charon's result.

**Clear Charon's Detection Cache:**

- Cache expires automatically after 1 hour
- Wait 60 minutes and try detection again
- Or restart Charon to clear in-memory cache

**Check DNS Provider Account:**

- Log into your DNS provider's control panel
- Verify the nameservers listed there
- Compare with Charon's detection result

**Use Manual Override:**

- If detection is consistently wrong
- Click **Select Manually**
- Choose correct provider
- Report issue via GitHub if pattern is incorrect

---

### Low Confidence Warning

**Symptom:** "DigitalOcean detected (Medium confidence)" or "Low confidence"

**What This Means:**

- Nameserver pattern match is partial or ambiguous
- Provider type identified, but match isn't strong
- Manual verification recommended before proceeding
- Not necessarily an error—just requires confirmation

**Confidence Levels Explained:**

| Level | Score | Meaning | Action |
|-------|-------|---------|--------|
| **High** | ≥80% | Strong, unambiguous match | Safe to auto-select |
| **Medium** | 50-79% | Probable match, some uncertainty | Verify before using |
| **Low** | 1-49% | Weak match, high uncertainty | Manual selection recommended |
| **None** | 0% | No match found | Must select manually |

**Recommended Actions:**

1. **Review Detected Provider Name**
   - Does it match your DNS provider?
   - Check against your account/records

2. **Check Nameservers List**
   - Examine the nameservers shown
   - Do they look familiar?
   - Do they match your provider's pattern?

3. **Verify Against DNS Account**
   - Log into your DNS provider
   - Compare nameservers
   - Confirm they match

4. **Decide:**
   - If match is correct: Click **Use [Provider]**
   - If uncertain: Click **Select Manually**

**Example:**

```
⚠ DigitalOcean detected (Medium confidence)

Nameservers:
• ns1.digitalocean.com
• ns2.some-other-service.com  ← Mixed nameservers

We detected DigitalOcean, but one nameserver doesn't match
the typical pattern. Please verify this is correct.
```

**Reason for Medium Confidence:** Only 1 of 2 nameservers matches DigitalOcean's pattern.

---

### Detection Takes Too Long

**Symptom:** Detection hangs or takes more than 5 seconds

**Possible Causes:**

- DNS server not responding
- Network latency or packet loss
- Domain's authoritative DNS servers offline

**Built-in Protections:**

- Detection timeout: 10 seconds maximum
- After timeout, detection fails gracefully
- Error message: "DNS lookup timeout"

**Solutions:**

- Wait for timeout (max 10 seconds)
- Check network connectivity
- Verify domain's DNS is operational
- Use manual provider selection

---

### Cache Showing Outdated Information

**Symptom:** Detection shows old provider after DNS migration

**Explanation:**

- Successful detections cached for 1 hour
- Improves performance for repeated requests
- May show outdated results during cache window

**Solutions:**

**Wait for Cache Expiration:**

- Cache automatically expires after 1 hour
- Try detection again after 60 minutes

**Restart Charon:**

- Cache is in-memory (not persistent)
- Restarting clears all cached detections
- Only necessary if you need immediate refresh

**Use Manual Selection:**

- Override cached detection
- Select correct provider manually
- Detection cache doesn't affect manual selection

---

## API Reference

### POST /api/v1/dns-providers/detect

**Detects DNS provider for a domain based on nameserver lookup**

**Authentication:** Required (Bearer token)

**Permissions:** Same as DNS provider management

**Request:**

```http
POST /api/v1/dns-providers/detect
Host: your-charon-instance
Authorization: Bearer YOUR_TOKEN
Content-Type: application/json

{
  "domain": "example.com"
}
```

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | Yes | Domain to detect (with or without `*.` wildcard) |

**Valid Domain Formats:**

- `example.com` → base domain
- `*.example.com` → wildcard (auto-stripped to base domain)
- `subdomain.example.com` → uses `example.com` for detection

**Response:** (200 OK)

```json
{
  "domain": "example.com",
  "detected": true,
  "provider_type": "cloudflare",
  "nameservers": [
    "ns1.cloudflare.com",
    "ns2.cloudflare.com"
  ],
  "confidence": "high",
  "suggested_provider": {
    "uuid": "550e8400-e29b-41d4-a716-446655440000",
    "name": "My Cloudflare Account",
    "provider_type": "cloudflare",
    "is_default": true,
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z"
  }
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `domain` | string | Base domain that was checked |
| `detected` | boolean | `true` if provider was identified, `false` otherwise |
| `provider_type` | string | Provider type identifier (e.g., `"cloudflare"`, `"route53"`) |
| `nameservers` | array | List of nameserver hostnames (always returned, even if empty) |
| `confidence` | string | Confidence level: `"high"`, `"medium"`, `"low"`, or `"none"` |
| `suggested_provider` | object\|null | Matching configured DNS provider, or `null` if none found |
| `error` | string | Error message (only present if detection failed) |

**Confidence Scoring:**

- **High (≥80%):** Most nameservers match pattern, strong confidence
- **Medium (50-79%):** Some nameservers match, partial confidence
- **Low (1-49%):** Few nameservers match, weak confidence
- **None (0%):** No nameservers match any pattern

**Error Responses:**

**400 Bad Request** - Invalid domain

```json
{
  "error": "domain is required"
}
```

**401 Unauthorized** - Missing or invalid token

```json
{
  "error": "Unauthorized"
}
```

**500 Internal Server Error** - Detection failure

```json
{
  "domain": "example.com",
  "detected": false,
  "nameservers": [],
  "confidence": "none",
  "error": "DNS lookup timeout: no nameservers found"
}
```

**Status Codes:**

- `200 OK` - Detection completed (success or failure)
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Authentication required or failed
- `500 Internal Server Error` - Unexpected server error

**Rate Limiting:**

- Detection results cached for 1 hour
- Repeated requests for same domain return cached result
- No explicit rate limit (DNS timeout provides natural throttling)

**Example Usage:**

```javascript
// JavaScript/Node.js
const detectDNSProvider = async (domain) => {
  const response = await fetch('https://charon.example.com/api/v1/dns-providers/detect', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ domain })
  });

  if (!response.ok) {
    throw new Error(`Detection failed: ${response.status}`);
  }

  const result = await response.json();

  if (result.confidence === 'high') {
    console.log(`✓ ${result.provider_type} detected with high confidence`);
    return result.suggested_provider;
  } else {
    console.log(`⚠ Low confidence detection, manual selection recommended`);
    return null;
  }
};
```

```python
# Python
import requests

def detect_dns_provider(domain, token):
    url = 'https://charon.example.com/api/v1/dns-providers/detect'
    headers = {
        'Authorization': f'Bearer {token}',
        'Content-Type': 'application/json'
    }
    payload = {'domain': domain}

    response = requests.post(url, json=payload, headers=headers)
    response.raise_for_status()

    result = response.json()

    if result['confidence'] == 'high':
        print(f"✓ {result['provider_type']} detected with high confidence")
        return result['suggested_provider']
    else:
        print(f"⚠ Low confidence detection")
        return None
```

```bash
# Bash with jq
detect_dns_provider() {
  local domain=$1
  local token=$2

  curl -s -X POST \
    "https://charon.example.com/api/v1/dns-providers/detect" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "{\"domain\": \"$domain\"}" | jq '
      if .confidence == "high" then
        "✓ \(.provider_type) detected (high confidence)"
      else
        "⚠ Low confidence: \(.confidence)"
      end
    '
}
```

---

### GET /api/v1/dns-providers/detection-patterns

**Returns built-in nameserver pattern database**

**Authentication:** Required (Bearer token)

**Permissions:** Same as DNS provider management

**Request:**

```http
GET /api/v1/dns-providers/detection-patterns
Host: your-charon-instance
Authorization: Bearer YOUR_TOKEN
```

**Response:** (200 OK)

```json
{
  "patterns": {
    "cloudflare.com": "cloudflare",
    "awsdns": "route53",
    "digitalocean.com": "digitalocean",
    "googledomains.com": "googleclouddns",
    "ns-cloud": "googleclouddns",
    "azure-dns": "azure",
    "registrar-servers.com": "namecheap",
    "domaincontrol.com": "godaddy",
    "hetzner.com": "hetzner",
    "hetzner.de": "hetzner",
    "vultr.com": "vultr",
    "dnsimple.com": "dnsimple"
  }
}
```

**Response Format:**

- `patterns` (object): Map of nameserver patterns to provider types
- **Keys:** Substring pattern to match in nameserver hostname (case-insensitive)
- **Values:** Provider type identifier used in Charon

**Pattern Matching:**

- Case-insensitive substring matching
- If any nameserver contains pattern, it's a match
- Multiple patterns can match the same provider (e.g., Google Cloud DNS)
- Score calculated based on percentage of nameservers matching

**Error Responses:**

**401 Unauthorized** - Missing or invalid token

```json
{
  "error": "Unauthorized"
}
```

**Status Codes:**

- `200 OK` - Patterns returned successfully
- `401 Unauthorized` - Authentication required or failed

**Example Usage:**

```bash
# Retrieve all patterns
curl https://charon.example.com/api/v1/dns-providers/detection-patterns \
  -H "Authorization: Bearer YOUR_TOKEN" | jq .
```

**Use Cases:**

- Building custom detection tools
- Debugging detection issues
- Understanding which providers are supported
- Verifying pattern database version

---

## Performance

### Detection Speed

**Typical Performance:**

- **First Detection:** 100-200ms (includes DNS lookup)
- **Cached Detection:** <1ms (from in-memory cache)
- **DNS Timeout:** 10 seconds maximum (prevents hanging)

**Performance Factors:**

| Factor | Impact | Notes |
|--------|--------|-------|
| DNS server latency | 50-150ms | External dependency |
| Pattern matching | <1ms | In-memory operation |
| Database lookup | 1-5ms | Optional, for provider suggestion |
| Cache hit | <1ms | In-memory hash map |
| Network latency | Varies | Between Charon and DNS servers |

**Performance Optimization:**

- Results cached for 1 hour
- Reduces repeated DNS lookups
- Cache hit rate typically 60-80%+ for active domains
- Pattern matching is O(1) hash map lookup

---

### Caching Behavior

**Cache Configuration:**

| Setting | Value | Rationale |
|---------|-------|-----------|
| **Success TTL** | 1 hour | Nameservers rarely change |
| **Failed TTL** | 5 minutes | Allow quick retry after transient failures |
| **Storage** | In-memory | Fast access, automatic cleanup |
| **Persistence** | None | Cache cleared on restart |

**Cache Key:** Base domain (e.g., `example.com`)

**Cache Invalidation:**

- Automatic expiration after TTL
- No manual invalidation API
- Restart Charon to clear all cached entries

**Cache Hit Scenarios:**

- Same domain detected multiple times
- Multiple wildcard proxies for same domain
- Repeated API calls within 1-hour window

**Cache Miss Scenarios:**

- First detection for a domain
- Cache entry expired (>1 hour old)
- Domain's DNS recently changed
- Charon restarted

**Performance Impact:**

- Cache hit: <1ms response time
- Cache miss: 100-200ms response time (DNS lookup required)
- Cache reduces DNS query load by ~80%

---

### Scalability

**System Limits:**

| Metric | Limit | Notes |
|--------|-------|-------|
| Concurrent detections | ~100s | Limited by DNS resolver |
| Cache size | Unbounded | In-memory, grows with unique domains |
| Memory per entry | ~1-2 KB | Includes result + metadata |
| DNS timeout | 10 seconds | Per-request maximum |

**Recommendations for High-Volume Usage:**

- Deploy Charon with adequate memory (cache can grow)
- Consider DNS server location/latency
- Monitor cache hit rate for optimization
- Use dedicated DNS resolver for production

---

## Security

### Authentication & Authorization

**Endpoint Security:**

- All detection endpoints require authentication
- Bearer token must be provided in `Authorization` header
- Same permission model as DNS provider management
- Unauthorized requests return `401 Unauthorized`

**Permission Requirements:**

- User must have access to DNS provider features
- No special permissions required for detection
- Detection doesn't expose sensitive credentials
- Only domain metadata returned (nameservers)

---

### Data Privacy

**What Charon Collects:**

- ✅ Domain name (from user input)
- ✅ Nameserver hostnames (from DNS lookup)
- ✅ Detection result (cached for 1 hour)

**What Charon Does NOT Collect:**

- ❌ DNS credentials or API keys
- ❌ Certificate private keys
- ❌ User browsing history
- ❌ DNS query contents
- ❌ Personal identifiable information (beyond domain ownership)

**Data Storage:**

- Detection results cached in-memory only
- No persistent storage of detection data
- Cache cleared on restart
- No logging of detected domains (unless debug logging enabled)

**Third-Party Access:**

- No data sent to third-party services
- DNS lookups go directly to configured DNS resolvers
- No analytics or telemetry for detection feature

---

### DNS Query Security

**Query Characteristics:**

- Standard DNS NS (nameserver) record lookups
- Uses system DNS resolver by default
- Respects DNS timeout (10 seconds)
- No recursive queries or zone transfers
- Read-only DNS operations

**Security Measures:**

- DNS timeout prevents hanging on unresponsive servers
- No user-controlled DNS servers (uses system config)
- Input validation on domain names
- Error handling for malformed responses

**Network Security:**

- DNS queries over UDP/TCP port 53
- No TLS/HTTPS for DNS (standard DNS protocol)
- Consider using DNS-over-HTTPS (DoH) in system resolver for privacy
- Firewall must allow outbound DNS (port 53)

---

### Attack Surface

**Potential Threats:**

| Threat | Mitigation | Risk Level |
|--------|------------|-----------|
| DNS spoofing | Use trusted DNS resolvers | Low |
| Cache poisoning | 1-hour TTL limits impact | Low |
| DoS via slow DNS | 10-second timeout | Low |
| Information disclosure | Only public DNS data returned | Very Low |
| Credential exposure | Detection doesn't access credentials | None |

**Security Best Practices:**

- Use trusted, secure DNS resolvers (e.g., 1.1.1.1, 8.8.8.8)
- Enable DNSSEC validation if possible
- Monitor detection error rates for anomalies
- Review detection logs for suspicious patterns

---

## Limitations

### 1. Built-in Providers Only

**Limitation:** Currently supports 10 major DNS providers

**Impact:**

- Custom DNS providers won't be auto-detected
- Niche/regional providers not in pattern database
- Self-hosted DNS servers not recognized

**Workaround:**

- Use manual provider selection
- Request provider pattern addition via GitHub issue
- Contribute pattern via pull request

**Future Enhancement:** User-configurable custom patterns

---

### 2. Shared Nameserver Patterns

**Limitation:** Some hosting providers use shared nameserver pools

**Impact:**

- Nameserver patterns may be ambiguous
- Detection may suggest incorrect provider
- Confidence scoring may be lower

**Example:**

- Some resellers use white-labeled nameservers
- Shared hosting platforms with generic nameserver names

**Workaround:**

- Verify detection result against your account
- Use manual selection if detection is incorrect
- Report ambiguous patterns for improvement

---

### 3. DNS Propagation Delays

**Limitation:** DNS changes take up to 48 hours to propagate globally

**Impact:**

- Detection may show old/outdated provider
- Recent migrations not immediately reflected
- Newly registered domains may fail detection

**Workaround:**

- Wait for DNS propagation to complete
- Check nameservers with `dig` or `nslookup`
- Use manual selection during migration period
- Re-run detection after propagation

---

### 4. Network Dependency

**Limitation:** Requires DNS connectivity to function

**Impact:**

- Offline/airgapped environments cannot use auto-detection
- Network issues cause detection failures
- DNS server outages prevent detection

**Workaround:**

- Use manual provider selection in offline environments
- Ensure DNS connectivity for auto-detection
- Detection failure doesn't block manual configuration

---

### 5. Cache Duration

**Limitation:** Results cached for 1 hour

**Impact:**

- Recent DNS changes not immediately reflected
- Cache may show outdated information
- No manual cache invalidation

**Workaround:**

- Wait 60 minutes for cache expiration
- Restart Charon to clear cache immediately
- Use manual selection to override cached result

**Rationale:** 1-hour cache significantly improves performance while nameservers rarely change

---

### 6. No Batch Detection

**Limitation:** Only one domain detected at a time

**Impact:**

- Cannot detect multiple domains in one request
- API requires separate call per domain
- Bulk operations require iteration

**Workaround:**

- Implement client-side batching
- Leverage cache for repeated domains
- Use async/parallel API calls

**Future Enhancement:** Batch detection endpoint planned

---

## Best Practices

### 1. Verify Detection Results

**Always Review Before Proceeding:**

- ✅ Check detected provider name matches your expectation
- ✅ Review nameserver list for accuracy
- ✅ Verify confidence level is acceptable
- ✅ Compare with your DNS account if uncertain

**Why:**

- Detection is not 100% accurate
- DNS configuration can be complex
- Wrong provider = certificate issuance failure

**Example Review Checklist:**

```
✓ Provider name: "Cloudflare" ← Correct?
✓ Nameservers: ns1.cloudflare.com ← Recognized?
✓ Confidence: "High" ← Acceptable?
✓ Matches DNS account? ← Verified!
```

---

### 2. High Confidence Only for Production

**Recommendation:** Only use auto-selection for "High" confidence detections

**Confidence Guidelines:**

| Environment | Minimum Confidence | Action |
|-------------|-------------------|--------|
| Production | High (≥80%) | Auto-select OK |
| Staging | Medium (≥50%) | Verify first |
| Development | Low/Any | Manual verify |

**Why:**

- Production certificate failures are costly
- High confidence = strong, unambiguous match
- Medium/Low = requires human verification

---

### 3. Keep Manual Override Available

**Always Provide Manual Selection:**

- Don't remove "Select Manually" button
- Auto-detection is a convenience, not requirement
- Users may know better than detection algorithm
- Edge cases always exist

**UI Pattern:**

```
✓ Cloudflare detected (High confidence)
[✓ Use Cloudflare]  [Select Manually]  ← Keep both options!
```

---

### 4. Test Before Production

**Test Detection on Development Domains First:**

```bash
# Test detection
curl -X POST https://charon-dev.internal/api/v1/dns-providers/detect \
  -H "Authorization: Bearer $DEV_TOKEN" \
  -d '{"domain": "dev.example.com"}'

# Verify result
# → Check provider type
# → Check confidence level
# → Compare with known DNS provider

# If successful, proceed to production
```

**Benefits:**

- Identify detection issues early
- Verify your DNS setup is detectable
- Test integration before production use

---

### 5. Monitor Detection Success Rates

**Track Metrics:**

- Detection success rate (detected vs. not detected)
- Confidence distribution (high/medium/low/none)
- Manual override rate (users choosing manual selection)
- Detection errors (timeouts, failures)

**Use Metrics to:**

- Identify common providers not in database
- Detect DNS configuration issues
- Improve pattern database
- Optimize cache hit rate

**Example Monitoring:**

```
Detection Stats (Last 7 Days):
- Total detections: 1,234
- Successful (high confidence): 987 (80%)
- Medium/Low confidence: 123 (10%)
- Failed/No match: 124 (10%)
- Manual overrides: 45 (3.6%)
```

---

### 6. Report Detection Issues

**Help Improve Detection:**

When detection fails or is incorrect:

1. ✅ Note the domain (if not sensitive)
2. ✅ Check actual nameservers: `dig NS domain.com +short`
3. ✅ Note expected provider
4. ✅ Note what Charon detected
5. ✅ Report via GitHub issue

**Example GitHub Issue:**

```markdown
**Title:** Detection fails for Linode DNS

**Description:**
- Domain: example.com
- Expected provider: Linode
- Detected provider: None
- Nameservers: ns1.linode.com, ns2.linode.com
- Confidence: None

**Suggested Fix:**
Add pattern: "linode.com" → "linode"
```

**Benefits:**

- Helps other users with same provider
- Improves detection accuracy
- Expands supported provider list

---

### 7. Cache Awareness

**Understand Caching Behavior:**

- ✅ First detection: 100-200ms
- ✅ Repeat detection (within 1 hour): <1ms
- ✅ After 1 hour: Fresh DNS lookup

**Considerations:**

- Don't rely on immediate updates after DNS changes
- Wait 60 minutes or restart Charon after migration
- Cache improves performance—embrace it!

**When Cache Matters:**

- DNS provider migration in progress
- Testing detection repeatedly
- Debugging detection issues

**Cache Doesn't Affect:**

- Manual provider selection
- Certificate issuance
- Existing proxy host configurations

---

## Future Enhancements

### Planned Features

**1. Custom Nameserver Pattern Definitions**

- Allow users to add custom provider patterns
- Define patterns via Web UI or configuration file
- Support for internal/private DNS providers
- Pattern validation and testing tools

**2. Detection History and Statistics**

- View past detection results
- Success/failure rates per provider
- Confidence distribution charts
- Most common providers in your environment

**3. Support for Additional DNS Providers**

- Add more regional providers
- Support for niche/specialized DNS services
- Community-contributed pattern library
- Automatic pattern updates

**4. Detection Caching Configuration**

- Configurable cache TTL (currently fixed at 1 hour)
- Per-provider cache settings
- Manual cache invalidation API
- Cache statistics dashboard

**5. Batch Domain Detection**

- Detect multiple domains in one API call
- Bulk import with auto-detection
- CSV upload with detection report
- Parallel detection processing

**6. Enhanced Confidence Scoring**

- Machine learning-based scoring
- Historical accuracy feedback
- Provider-specific confidence thresholds
- Confidence explanation details

**7. Detection Webhooks**

- Notify external systems of detection results
- Integrate with automation workflows
- Detection event logging
- Real-time detection monitoring

---

### Community Contributions

**We Welcome:**

- 🌟 New provider pattern additions
- 🐛 Bug reports for incorrect detections
- 💡 Feature requests and ideas
- 📝 Documentation improvements
- 🧪 Test cases for edge scenarios

**How to Contribute:**

**Add a Provider Pattern:**

```bash
# 1. Fork repository
# 2. Edit: backend/internal/services/dns_detection_service.go
# 3. Add pattern to BuiltInNameservers map:

var BuiltInNameservers = map[string]string{
    // ...existing patterns...

    // Your new provider
    "newprovider.com": "newprovider",
}

# 4. Add test case to: backend/internal/services/dns_detection_service_test.go
# 5. Submit pull request
```

**Report Detection Issues:**

- GitHub Issues: <https://github.com/Wikid82/Charon/issues>
- Label: `enhancement`, `dns-detection`
- Provide: Domain example, nameservers, expected provider

**Share Use Cases:**

- How are you using auto-detection?
- What workflows does it enable?
- What features would be helpful?

---

### Feedback Welcome

**Help Us Improve:**

- Share your experience with auto-detection
- Report detection accuracy issues
- Suggest new provider patterns
- Request feature enhancements

**Contact:**

- GitHub Issues: <https://github.com/Wikid82/Charon/issues>
- GitHub Discussions: <https://github.com/Wikid82/Charon/discussions>
- Documentation: <https://docs.charon.example.com>
- Community: <https://community.charon.example.com>

---

## Related Documentation

- **[DNS Challenge Support](dns-challenge-support.md)** - Core wildcard certificate feature using DNS-01 challenges
- **[DNS Provider Configuration](dns-providers.md)** - Setting up and managing DNS provider credentials
- **[Multi-Credential Management](multi-credential.md)** - Advanced multi-provider and multi-account setups
- **[API Reference](../api/README.md)** - Complete Charon API documentation
- **[Security Best Practices](../security/README.md)** - Security guidelines for Charon deployment

---

## Changelog

### Version 1.0.0 (January 2026)

**Initial Release**

- ✨ DNS provider auto-detection for 10+ major providers
- 🚀 Web UI integration with ProxyHost form
- 🔌 RESTful API endpoints (`/detect`, `/detection-patterns`)
- ⚡ Performance: 100-200ms typical detection time
- 🗄️ 1-hour in-memory caching
- 🎯 Confidence scoring (high/medium/low/none)
- 🧪 Comprehensive test coverage (86.3% backend, 85.67% frontend)
- 📚 Full API and user documentation
- 🔒 Security: Authentication required, no credential exposure
- ♿ Accessibility: ARIA labels, keyboard navigation

**Supported Providers:**

- Cloudflare
- Amazon Route 53
- DigitalOcean
- Google Cloud DNS
- Microsoft Azure DNS
- Namecheap
- GoDaddy
- Hetzner
- Vultr
- DNSimple

**Technical Details:**

- Pattern-based nameserver matching
- Automatic wildcard domain normalization
- Thread-safe cache implementation
- 10-second DNS lookup timeout
- Graceful fallback to manual selection
- Zero breaking changes to existing functionality

---

## FAQ

**Q: Is auto-detection required to use wildcard certificates?**
A: No, it's optional. Manual provider selection always works.

**Q: How accurate is auto-detection?**
A: Very accurate for the 10+ built-in providers. Confidence scoring helps identify uncertain matches.

**Q: What happens if my provider isn't detected?**
A: Simply select your provider manually—detection is a convenience feature, not a requirement.

**Q: Can I add support for my custom DNS provider?**
A: Currently, only built-in patterns are supported. Custom patterns are planned for future releases. You can contribute patterns via GitHub.

**Q: Does detection work offline?**
A: No, detection requires DNS connectivity. Use manual selection in offline/airgapped environments.

**Q: How long are results cached?**
A: 1 hour for successful detections, 5 minutes for failures.

**Q: Can I clear the detection cache?**
A: Cache expires automatically. Restart Charon to clear immediately.

**Q: Does detection expose my DNS credentials?**
A: No, detection only looks up public nameserver records. No credentials are accessed.

**Q: What if detection suggests the wrong provider?**
A: Use "Select Manually" to override. Report the issue on GitHub to help improve accuracy.

**Q: How fast is detection?**
A: Typically 100-200ms for first detection, <1ms for cached results.

---

## Support

**Questions or Issues?**

- 📖 Documentation: <https://docs.charon.example.com>
- 🐛 GitHub Issues: <https://github.com/Wikid82/Charon/issues>
- 💬 GitHub Discussions: <https://github.com/Wikid82/Charon/discussions>
- 👥 Community Forum: <https://community.charon.example.com>

**Feature Requests:**

- Submit via GitHub Issues with label `enhancement`
- Describe your use case and desired functionality
- Include examples and expected behavior

**Bug Reports:**

- Submit via GitHub Issues with label `bug`
- Include: Domain (if not sensitive), nameservers, expected vs. actual result
- Attach detection API response if available

---

**Thank you for using Charon! 🚀**
