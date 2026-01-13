# Multi-Credential per DNS Provider

## Feature Overview

### What is Multi-Credential Support?

Multi-Credential per Provider is an advanced feature that allows you to configure multiple sets of API credentials for the same DNS provider. Instead of using a single API key for all domains managed by a provider (e.g., Cloudflare), you can configure different credentials for different DNS zones.

### Why Use Multi-Credentials?

**Security Benefits:**

- **Zone-level Isolation**: Compromise of one credential doesn't expose all your domains
- **Least Privilege Principle**: Each credential can have minimal permissions for only the zones it manages
- **Independent Rotation**: Rotate credentials for specific zones without affecting others
- **Audit Trail**: Track which credentials were used for certificate operations

**Business Use Cases:**

- **Managed Service Providers (MSPs)**: Use separate customer-specific credentials for each client's domains
- **Large Enterprises**: Isolate credentials by department, environment, or geographic region
- **Multi-Tenant Platforms**: Provide credential isolation between tenants
- **Compliance Requirements**: Meet security standards requiring credential segregation

### Single vs Multi-Credential Architecture

```
Single Credential Mode:
┌─────────────────────────┐
│  Cloudflare Provider    │
│  API Key: xyz123        │
└───────────┬─────────────┘
            │
    ┌───────┴────────┬────────────┬──────────────┐
    │                │            │              │
example.com    customer-a.com  *.dev.com   acme.org
```

```
Multi-Credential Mode:
┌──────────────────────────────────────────────────┐
│           Cloudflare Provider                    │
├──────────────────────────────────────────────────┤
│ Credential 1: "Production"                       │
│   Zone Filter: example.com                       │
│   ├─→ example.com                                │
│   └─→ www.example.com                            │
├──────────────────────────────────────────────────┤
│ Credential 2: "Customer A"                       │
│   Zone Filter: *.customer-a.com                  │
│   └─→ *.customer-a.com                           │
├──────────────────────────────────────────────────┤
│ Credential 3: "Development"                      │
│   Zone Filter: *.dev.com                         │
│   └─→ *.dev.com                                  │
├──────────────────────────────────────────────────┤
│ Credential 4: "Catch-all"                        │
│   Zone Filter: (empty - matches anything else)  │
│   └─→ acme.org, other.net, etc.                 │
└──────────────────────────────────────────────────┘
```

## When to Use Multi-Credentials

### Decision Criteria

**Use Multi-Credentials When:**

- You manage domains for multiple customers or tenants
- You need credential isolation for security or compliance
- Different teams or departments manage different zones
- You want to limit blast radius of credential compromise
- You have different security postures for different environments (prod/staging/dev)
- You need independent credential rotation schedules

**Use Single Credential When:**

- You manage a small number of domains under one organization
- All domains have the same security requirements
- Simpler management is preferred over isolation
- You're just getting started with Charon

### Comparison Matrix

| Aspect | Single Credential | Multi-Credential |
|--------|------------------|------------------|
| **Security Isolation** | ❌ All zones use same key | ✅ Per-zone isolation |
| **Blast Radius** | ❌ High (all zones affected) | ✅ Limited to filtered zones |
| **Setup Complexity** | ✅ Simple | ⚠️ Moderate |
| **Credential Rotation** | ❌ Affects all zones | ✅ Independent per zone |
| **Audit Granularity** | ⚠️ Provider-level only | ✅ Credential-level detail |
| **Multi-Tenancy** | ❌ Not suitable | ✅ Ideal |
| **Best For** | Small deployments | MSPs, enterprises, multi-tenant |

## Enabling Multi-Credential Mode

### Prerequisites

- Charon v1.3.0 or later
- Admin access to the Charon dashboard
- DNS provider account with API access
- (Optional) Multiple API keys already generated at your DNS provider

### Step-by-Step Enable Process

1. **Navigate to DNS Provider Settings**
   - Go to **Settings** → **DNS Providers**
   - Locate the provider you want to enable multi-credential for (e.g., Cloudflare)

2. **Click "Enable Multi-Credential"**
   - Click the **Enable Multi-Credential** button next to the provider
   - A confirmation dialog will appear

3. **Review Migration Impact**

   ```
   ⚠️ IMPORTANT: This action will:
   - Convert your existing provider credential into a "catch-all" credential
   - Preserve all existing proxy host configurations
   - Enable credential management UI for this provider
   - This change is reversible (you can disable and revert)
   ```

4. **Confirm Enable**
   - Click **Enable** to proceed
   - The provider will now show "Multi-Credential Mode: Enabled"

5. **Verify Migration**
   - Your existing credential is now listed as a credential with no zone filter (catch-all)
   - All existing proxy hosts continue to work without interruption
   - You can now add additional credentials

### What Happens During Migration

1. **Existing Configuration Preserved**: Your current API key/token remains active
2. **Automatic Credential Creation**: The existing credential is converted to a credential entry with:
   - Name: "{Provider} Primary Credential"
   - Zone Filter: Empty (matches all zones)
   - Description: "Migrated from single-credential mode"
3. **Zero Downtime**: All certificate operations continue without interruption
4. **Backward Compatible**: If you disable multi-credential mode, you revert to the original setup

### Backward Compatibility

- **Disabling Multi-Credential**: You can disable multi-credential mode by clicking **Disable Multi-Credential**
- **Reversion**: Disabling converts the first credential back to the provider's primary credential
- **Data Loss**: Other credentials will be retained in the database but won't be used
- **Re-enabling**: You can re-enable multi-credential mode at any time without data loss

## Managing Credentials

### Accessing Credential Management

1. Navigate to **Settings** → **DNS Providers**
2. Find your provider with "Multi-Credential Mode: Enabled"
3. Click **Manage Credentials**
4. The credential management interface displays all credentials for this provider

### Adding a New Credential

#### Step 1: Click "Add Credential"

Click the **Add Credential** button in the credential management interface.

#### Step 2: Fill in Credential Details

**Required Fields:**

- **Credential Name**: A descriptive name (e.g., "Customer A Production", "US Zones", "Dev Environment")
  - Must be unique within the provider
  - Recommended: Use descriptive names that indicate purpose or zone scope

- **API Credentials**: Provider-specific authentication fields
  - **Cloudflare**: API Token or Global API Key + Email
  - **Route53**: Access Key ID + Secret Access Key
  - **DigitalOcean**: API Token
  - (Refer to provider-specific guides for required fields)

**Optional Fields:**

- **Description**: Additional notes about the credential's purpose
- **Zone Filter**: Comma-separated list of zones this credential manages (see Zone Filter Configuration below)

#### Step 3: Configure Zone Filter

The zone filter determines which domains this credential will be used for:

**Option 1: Exact Domain Match**

```
Zone Filter: example.com
Matches: example.com, www.example.com, api.example.com
Does NOT Match: subdomain.example.com.au, example.org
```

**Option 2: Wildcard Match**

```
Zone Filter: *.customer-a.com
Matches: shop.customer-a.com, api.customer-a.com, *.customer-a.com
Does NOT Match: customer-a.com (root), customer-b.com
```

**Option 3: Multiple Zones (Comma-Separated)**

```
Zone Filter: example.com,api.example.org,*.dev.example.net
Matches:
  - example.com and all subdomains
  - api.example.org and all subdomains under api.example.org
  - *.dev.example.net (all subdomains of dev.example.net)
```

**Option 4: Catch-All (Empty Filter)**

```
Zone Filter: (leave empty)
Matches: Any domain not matched by other credentials
Use Case: Fallback credential for miscellaneous domains
```

#### Step 4: Test the Credential (Recommended)

1. Click **Test Credential** before saving
2. Charon will attempt to authenticate with the DNS provider using the supplied credentials
3. If successful, you'll see: ✅ "Credential validated successfully"
4. If failed, review the error message and correct the credentials

#### Step 5: Save the Credential

- Click **Save Credential**
- The credential is now active and will be used for matching domains

### Zone Filter Configuration

#### Syntax Rules

- **Comma-separated**: Separate multiple patterns with commas (no spaces)
- **Case-insensitive**: `Example.com` matches `example.com`
- **Wildcard prefix**: Use `*.` at the start for subdomain matching
- **No regex**: Only exact and wildcard matches are supported
- **No trailing dots**: Don't use `example.com.` (trailing dot is stripped)

#### Examples

| Zone Filter | Matches | Does NOT Match |
|-------------|---------|----------------|
| `example.com` | `example.com`, `www.example.com`, `api.example.com` | `example.org`, `sub.example.com.au` |
| `*.customer-a.com` | `shop.customer-a.com`, `api.customer-a.com` | `customer-a.com` (root) |
| `*.staging.example.com` | `app.staging.example.com`, `api.staging.example.com` | `staging.example.com`, `prod.example.com` |
| `example.com,example.org` | Both `example.com` and `example.org` domains | `example.net` |
| _(empty)_ | Any domain not matched by other credentials | _(none - this is catch-all)_ |

#### Validation Rules

When saving a credential, Charon validates:

- ✅ Zone filter syntax is correct
- ✅ No duplicate exact matches across credentials
- ⚠️ Warning if multiple wildcard patterns could overlap
- ✅ At most one credential per provider can have an empty zone filter (catch-all)

### Editing Credentials

1. In the credential management interface, click **Edit** next to the credential
2. Modify any field (name, description, credentials, zone filter)
3. Click **Test Credential** to validate changes
4. Click **Save Changes**

**⚠️ Important Notes:**

- Changing zone filters may affect which credential is used for existing proxy hosts
- Charon will re-evaluate credential matching for all proxy hosts after the change
- Consider testing in a non-production environment first if making significant changes

### Testing Credentials

You can test credentials at any time to verify they still work:

1. Click **Test** next to the credential in the management interface
2. Charon will attempt a test DNS record lookup or API call
3. Results:
   - ✅ **Success**: Credential is valid and working
   - ❌ **Failed**: Credential is invalid or has insufficient permissions
   - ⚠️ **Warning**: Credential works but may have limited permissions

### Deleting Credentials

1. Click **Delete** next to the credential
2. Charon will check if any proxy hosts are currently using this credential
3. **If in use**: You'll be warned and must migrate those proxy hosts to another credential first
4. **If not in use**: Confirm deletion and the credential will be removed

**⚠️ Warning**: Deleting a credential that is actively in use for certificate operations will cause certificate renewals to fail. Always ensure proxy hosts are migrated to another credential before deletion.

## Zone Matching Rules

### How Zone Matching Works

When Charon needs to issue or renew a certificate for a domain, it selects a credential using this process:

```
1. Extract DNS zone from the domain
   Example: For "www.api.example.com", zone is "example.com"

2. Query all credentials for the provider (e.g., Cloudflare)

3. Match credentials against the zone using priority order:
   a. Exact match (highest priority)
   b. Wildcard match
   c. Catch-all (empty zone filter) (lowest priority)

4. Return the first matching credential

5. Use the credential to perform DNS-01 challenge
```

### Priority Order

Credentials are evaluated in this order:

**1. Exact Match (Highest Priority)**

```
Zone Filter: example.com
Domain: www.example.com → Zone: example.com → ✅ MATCH
```

**2. Wildcard Match**

```
Zone Filter: *.customer-a.com
Domain: shop.customer-a.com → Zone: customer-a.com → ✅ MATCH (after exact check fails)
```

**3. Catch-All (Lowest Priority)**

```
Zone Filter: (empty)
Domain: anything.com → Zone: anything.com → ✅ MATCH (if no exact or wildcard matches)
```

### Matching Examples

#### Example 1: MSP with Multiple Customers

**Configured Credentials:**

```
1. Name: "Customer A Production"
   Zone Filter: *.customer-a.com
   Priority: Wildcard

2. Name: "Customer B Production"
   Zone Filter: *.customer-b.com
   Priority: Wildcard

3. Name: "Catch-all"
   Zone Filter: (empty)
   Priority: Catch-all
```

**Domain Matching:**

- `shop.customer-a.com` → Credential 1 ("Customer A Production")
- `api.customer-b.com` → Credential 2 ("Customer B Production")
- `example.com` → Credential 3 ("Catch-all")
- `random.org` → Credential 3 ("Catch-all")

#### Example 2: Environment Separation

**Configured Credentials:**

```
1. Name: "Production"
   Zone Filter: example.com
   Priority: Exact

2. Name: "Staging"
   Zone Filter: *.staging.example.com
   Priority: Wildcard

3. Name: "Development"
   Zone Filter: *.dev.example.com
   Priority: Wildcard
```

**Domain Matching:**

- `www.example.com` → Credential 1 ("Production")
- `api.example.com` → Credential 1 ("Production")
- `app.staging.example.com` → Credential 2 ("Staging")
- `api.dev.example.com` → Credential 3 ("Development")
- `staging.example.com` → Credential 1 ("Production") - root of staging doesn't match `*.staging.example.com`

#### Example 3: Geographic Separation

**Configured Credentials:**

```
1. Name: "US Zones"
   Zone Filter: *.us.example.com
   Priority: Wildcard

2. Name: "EU Zones"
   Zone Filter: *.eu.example.com
   Priority: Wildcard

3. Name: "APAC Zones"
   Zone Filter: *.apac.example.com
   Priority: Wildcard
```

**Domain Matching:**

- `shop.us.example.com` → Credential 1 ("US Zones")
- `api.eu.example.com` → Credential 2 ("EU Zones")
- `portal.apac.example.com` → Credential 3 ("APAC Zones")
- `www.example.com` → ❌ NO MATCH (no catch-all defined)

### Overlapping Patterns

**⚠️ What Happens When Patterns Overlap?**

If multiple credentials could match the same zone, Charon uses **first match** based on priority order:

**Example:**

```
Credential A: Zone Filter: example.com (Exact)
Credential B: Zone Filter: *.example.com (Wildcard)

Domain: www.example.com
Zone: example.com

Match Process:
1. Check Exact: Credential A matches "example.com" → ✅ Use Credential A
2. (Wildcard check not reached)
```

**Best Practice**: Avoid overlapping patterns when possible. Design zone filters to be mutually exclusive.

### Troubleshooting Zone Matching

#### Issue: Domain doesn't match any credential

**Symptoms:**

- Certificate issuance fails with "No matching credential for zone"
- Error message: `No credential found for provider 'cloudflare' and zone 'example.com'`

**Solutions:**

1. **Add a catch-all credential**: Create a credential with an empty zone filter
2. **Add specific credential**: Create a credential with zone filter matching your domain
3. **Check zone extraction**: Ensure Charon is correctly extracting the zone from your domain

#### Issue: Wrong credential is being used

**Symptoms:**

- Expected credential "Production" but "Catch-all" is being used
- Certificate issued but not with the intended credential

**Solutions:**

1. **Check zone filter syntax**: Verify your zone filters are correctly configured
2. **Check priority order**: Exact matches override wildcards; ensure your exact match is configured
3. **Review audit logs**: Check which credential was actually selected and why

#### Issue: Zone filter validation error

**Symptoms:**

- Error: "Invalid zone filter format"
- Error: "Zone filter 'example.com' conflicts with existing credential"

**Solutions:**

1. **Check syntax**: Ensure no spaces, only commas separating patterns
2. **Check for duplicates**: Ensure no two credentials have the exact same zone filter pattern
3. **Review wildcard syntax**: Wildcards must be `*.domain.com`, not `*domain.com`

## Creating Proxy Hosts with Multi-Credentials

### Automatic Credential Selection

When you create a proxy host with multi-credential mode enabled:

1. **You don't select a credential** - Charon selects automatically
2. **Zone matching** - Charon extracts the DNS zone from your domain
3. **Credential lookup** - Charon finds the best matching credential using zone matching rules
4. **Certificate issuance** - The selected credential is used for DNS-01 challenge

### Step-by-Step Process

1. **Create Proxy Host as Normal**
   - Go to **Proxy Hosts** → **Add Proxy Host**
   - Enter domain name (e.g., `shop.customer-a.com`)
   - Configure forward host/port and other settings
   - Enable SSL/TLS and select Let's Encrypt

2. **Charon Selects Credential Automatically**
   - Charon extracts zone: `customer-a.com`
   - Searches for matching credentials for the DNS provider
   - Finds: "Customer A Production" (Zone Filter: `*.customer-a.com`)
   - Uses this credential for certificate issuance

3. **Certificate Issuance**
   - Charon requests certificate from Let's Encrypt
   - Uses selected credential to create DNS TXT record for `_acme-challenge.shop.customer-a.com`
   - Completes DNS-01 challenge
   - Certificate is issued

4. **View Selected Credential**
   - After creation, view the proxy host details
   - The **Certificate** section shows: "Issued using credential: Customer A Production"

### Viewing Which Credential Was Used

**Method 1: Proxy Host Details**

1. Open the proxy host from the dashboard
2. In the **SSL/TLS** section, look for:

   ```
   Certificate: Active (Expires: 2026-04-01)
   Credential Used: Customer A Production (Cloudflare)
   Last Renewed: 2026-01-02 14:30 UTC
   ```

**Method 2: Audit Logs**

1. Go to **Settings** → **Audit Logs**
2. Filter by: `Action: certificate_issued` or `Action: certificate_renewed`
3. View log entry:

   ```json
   {
     "timestamp": "2026-01-02T14:30:00Z",
     "action": "certificate_issued",
     "domain": "shop.customer-a.com",
     "provider": "cloudflare",
     "credential_id": 42,
     "credential_name": "Customer A Production",
     "user": "admin@example.com",
     "result": "success"
   }
   ```

**Method 3: Credential Statistics**

1. Go to **Settings** → **DNS Providers** → **Manage Credentials**
2. Each credential shows:
   - **Usage Count**: Number of domains using this credential
   - **Last Used**: Timestamp of last certificate operation
   - **Success Rate**: Success/failure ratio

### Troubleshooting Certificate Issuance

#### Issue: Certificate issuance fails with "No matching credential"

**Error Message:**

```
Failed to issue certificate for shop.customer-a.com:
No credential found for provider 'cloudflare' and zone 'customer-a.com'
```

**Solution:**

1. Check DNS provider has multi-credential enabled
2. Verify a credential exists with zone filter matching `customer-a.com`
3. Add a credential with zone filter: `*.customer-a.com` or use catch-all

#### Issue: Certificate issuance fails with "API authentication failed"

**Error Message:**

```
Failed to issue certificate for shop.customer-a.com:
Cloudflare API returned 403: Invalid credentials
```

**Solution:**

1. Test the credential being used: **Manage Credentials** → **Test**
2. Verify API token/key is still valid in your DNS provider dashboard
3. Check API token has correct permissions (`Zone:DNS:Edit`)
4. Update the credential with valid API credentials

#### Issue: Wrong credential is being used

**Symptoms:**

- Certificate issued successfully but with unexpected credential
- Audit logs show different credential than expected

**Solution:**

1. Review zone filter configuration for all credentials
2. Check priority order (exact > wildcard > catch-all)
3. Ensure your expected credential has the most specific zone filter
4. Test zone matching logic in **Manage Credentials** interface

## Credential Organization Best Practices

### Naming Conventions

**Recommended Naming Patterns:**

**By Customer/Tenant:**

```
- "Customer A - Production"
- "Customer B - Staging"
- "Tenant XYZ - All Zones"
```

**By Environment:**

```
- "Production Zones"
- "Staging Zones"
- "Development Zones"
```

**By Department:**

```
- "Marketing - example.com"
- "Engineering - api.example.com"
- "Sales - shop.example.com"
```

**By Geography:**

```
- "US East Zones"
- "EU West Zones"
- "APAC Zones"
```

### Zone Filter Strategies

#### Strategy 1: Exact Domain Per Credential

**Use Case:** Small number of high-value domains

**Example:**

```
Credential: "example.com Primary"
Zone Filter: example.com

Credential: "api.example.org"
Zone Filter: api.example.org
```

**Pros:**

- Maximum specificity
- Easy to understand
- Clear audit trail

**Cons:**

- Requires one credential per domain
- Not scalable for many domains

#### Strategy 2: Wildcard by Namespace

**Use Case:** Logical grouping of subdomains

**Example:**

```
Credential: "Customer Zones"
Zone Filter: *.customers.example.com

Credential: "Internal Services"
Zone Filter: *.internal.example.com
```

**Pros:**

- Scalable for many subdomains
- Clear organizational boundaries
- Reduces credential count

**Cons:**

- Broader scope than exact match
- Requires careful namespace planning

#### Strategy 3: Hybrid Approach

**Use Case:** Most common for production deployments

**Example:**

```
1. Exact matches for critical domains:
   - "Production Root" → example.com

2. Wildcards for namespaces:
   - "Customer A" → *.customer-a.com
   - "Customer B" → *.customer-b.com
   - "Staging" → *.staging.example.com

3. Catch-all for miscellaneous:
   - "Legacy & Misc" → (empty)
```

**Pros:**

- Balance of specificity and scalability
- Flexible and maintainable
- Handles edge cases

**Cons:**

- More credentials to manage
- Requires understanding of priority order

### When to Use Catch-All Credentials

**✅ Good Use Cases:**

1. **Proof-of-Concept/Testing**: Start with catch-all, refine later
2. **Backward Compatibility**: Ensure existing domains continue working during migration
3. **Miscellaneous Domains**: Handle legacy or one-off domains
4. **Gradual Migration**: Add specific credentials over time while catch-all handles the rest

**❌ Avoid Catch-All When:**

1. **High-Security Environments**: Catch-all defeats purpose of zone isolation
2. **Multi-Tenancy**: Each tenant should have explicit credentials
3. **Compliance**: Regulations may require explicit credential assignment
4. **Credential Rotation**: Catch-all makes rotation harder

### Credential Rotation Strategy

**Best Practice Rotation Schedule:**

- **High-Risk Credentials** (catch-all, root domains): Every 30 days
- **Production Credentials**: Every 90 days
- **Staging/Development**: Every 180 days
- **Test Credentials**: Every 365 days or as needed

**Rotation Process:**

1. **Generate New Credentials** at DNS provider
2. **Add New Credential** in Charon with same zone filter
3. **Test New Credential** - verify it works
4. **Update Zone Filter** of old credential to `__deprecated__` (forces Charon to use new credential)
5. **Wait for Certificate Renewals** - monitor for 7-30 days
6. **Delete Old Credential** once confirmed new credential is working

**Automation:**

- Use provider API to programmatically generate new credentials
- Use Charon API to add/update credentials
- Schedule rotation using cron or CI/CD pipeline
- Monitor audit logs for credential usage

## Monitoring and Maintenance

### Viewing Credential Usage Statistics

**Dashboard View:**

1. Navigate to **Settings** → **DNS Providers**
2. For each provider with multi-credential enabled, click **View Statistics**
3. Dashboard shows:
   - Total credentials configured
   - Active credentials (used in last 30 days)
   - Inactive credentials (not used in last 90 days)
   - Top credentials by usage

**Per-Credential View:**

1. Go to **Settings** → **DNS Providers** → **Manage Credentials**
2. Each credential displays:

```
┌─────────────────────────────────────────────────┐
│ Customer A Production                           │
│ Zone Filter: *.customer-a.com                   │
├─────────────────────────────────────────────────┤
│ Domains Using: 12                               │
│ Success Count: 156                              │
│ Failure Count: 2                                │
│ Success Rate: 98.7%                             │
│ Last Used: 2026-01-03 10:45 UTC                 │
│ Last Success: 2026-01-03 10:45 UTC              │
│ Last Failure: 2025-12-28 03:12 UTC              │
├─────────────────────────────────────────────────┤
│ [Test] [Edit] [View Domains] [Delete]          │
└─────────────────────────────────────────────────┘
```

### Success/Failure Counts

**Success Count**: Number of successful certificate operations (issuance, renewal) using this credential

**Failure Count**: Number of failed certificate operations

**Success Rate**: Percentage of successful operations (Success / (Success + Failure) × 100%)

**⚠️ Low Success Rate Alert:**

- If success rate drops below 90%, investigate immediately
- Common causes: expired API token, insufficient permissions, DNS provider API issues
- Click **Test Credential** to diagnose

### Last Used Timestamps

**Last Used**: Timestamp of the most recent certificate operation (success or failure)

**Last Success**: Timestamp of the most recent successful operation

**Last Failure**: Timestamp of the most recent failed operation

**Use Cases:**

- **Identify Unused Credentials**: If "Last Used" is > 90 days ago, consider removing
- **Credential Rotation**: Track when credentials were last active
- **Incident Response**: Correlate failures with outages or credential changes

### Audit Trail for Credential Operations

**Viewing Audit Logs:**

1. Go to **Settings** → **Audit Logs**
2. Filter by:
   - **Action Type**: `credential_created`, `credential_updated`, `credential_deleted`, `certificate_issued`, `certificate_renewed`
   - **Provider**: Filter by DNS provider
   - **User**: Filter by who performed the action

**Log Entry Example:**

```json
{
  "timestamp": "2026-01-04T15:30:00Z",
  "action": "credential_created",
  "resource_type": "dns_credential",
  "resource_id": 42,
  "resource_name": "Customer A Production",
  "provider": "cloudflare",
  "zone_filter": "*.customer-a.com",
  "user": "admin@example.com",
  "ip_address": "192.168.1.100",
  "details": {
    "description": "Credential for Customer A production domains",
    "created_via": "web_ui"
  }
}
```

**Exported Logs:**

- Export to CSV or JSON for external analysis
- Integrate with SIEM (Security Information and Event Management) systems
- Use for compliance reporting and security audits

## Troubleshooting

### Common Issues and Solutions

#### Issue 1: No matching credential for domain

**Symptoms:**

- Certificate issuance fails
- Error: `No credential found for provider 'cloudflare' and zone 'example.com'`
- Proxy host shows certificate status: "Failed"

**Root Causes:**

1. No credential configured for the DNS zone
2. Zone filter doesn't match the domain's zone
3. Multi-credential mode not enabled

**Solutions:**

**Step 1: Verify Multi-Credential Mode is Enabled**

```
Settings → DNS Providers → Check "Multi-Credential Mode: Enabled"
```

**Step 2: Check Existing Credentials**

```
Settings → DNS Providers → Manage Credentials
Review zone filters for all credentials
```

**Step 3: Add Missing Credential or Catch-All**

**Option A: Add Specific Credential**

```
Credential Name: example.com Production
Zone Filter: example.com
```

**Option B: Add Catch-All**

```
Credential Name: Catch-All
Zone Filter: (leave empty)
```

**Step 4: Retry Certificate Issuance**

```
Proxy Hosts → Select proxy host → SSL/TLS → Renew Certificate
```

#### Issue 2: Certificate issuance fails with API error

**Symptoms:**

- Certificate issuance fails
- Error: `Cloudflare API returned 403: Invalid credentials` or similar
- Credential test fails

**Root Causes:**

1. API token/key expired or revoked
2. Insufficient API permissions
3. DNS provider account suspended
4. Rate limiting or API quota exceeded

**Solutions:**

**Step 1: Test the Credential**

```
Settings → DNS Providers → Manage Credentials → Click "Test" next to credential
```

**Step 2: Check API Token Validity**

- Log in to your DNS provider dashboard (e.g., Cloudflare)
- Navigate to API Tokens
- Verify token is active and not expired
- Check token permissions: `Zone:DNS:Edit` permission required

**Step 3: Regenerate API Token**

- Generate new API token at DNS provider
- Update credential in Charon:

  ```
  Settings → DNS Providers → Manage Credentials → Edit → Update API credentials → Test → Save
  ```

**Step 4: Check API Rate Limits**

- Review DNS provider's rate limit documentation
- Check if you've exceeded API quotas
- Wait for rate limit to reset (typically hourly)
- Consider spreading certificate operations over time

**Step 5: Retry Certificate Issuance**

```
Proxy Hosts → Select proxy host → SSL/TLS → Renew Certificate
```

#### Issue 3: Zone filter validation error

**Symptoms:**

- Cannot save credential
- Error: `Invalid zone filter format: 'example..com'`
- Error: `Zone filter 'example.com' conflicts with existing credential`

**Root Causes:**

1. Syntax error in zone filter (typo, invalid characters)
2. Duplicate zone filter across multiple credentials
3. Conflicting exact and wildcard patterns

**Solutions:**

**Step 1: Check Syntax**

**Valid Formats:**

```
✅ example.com
✅ *.customer-a.com
✅ example.com,api.example.org
✅ *.staging.example.com,*.dev.example.com
✅ (empty - catch-all)
```

**Invalid Formats:**

```
❌ example..com (double dot)
❌ example.com. (trailing dot)
❌ *customer-a.com (missing dot after asterisk)
❌ example.com, api.example.org (space after comma)
❌ example.com; api.example.org (semicolon instead of comma)
```

**Step 2: Check for Duplicates**

- Review all credentials for the provider
- Ensure no two credentials have the exact same zone filter pattern
- If duplicate found, edit one credential to have a different zone filter

**Step 3: Resolve Conflicts**

- If you have both `example.com` and `*.example.com`, this is allowed but may cause confusion
- Ensure you understand priority order: exact match takes precedence

**Step 4: Save Again**

- After fixing syntax/duplicates, click **Save Credential**

#### Issue 4: Wrong credential is being used

**Symptoms:**

- Certificate issued successfully but audit logs show unexpected credential
- Credential statistics don't match expectations
- Security/compliance concern about which credential was used

**Root Causes:**

1. Zone filter misconfiguration (too broad or too narrow)
2. Misunderstanding of zone matching priority
3. Overlapping patterns causing unexpected matches

**Solutions:**

**Step 1: Review Zone Matching Logic**

```
Priority Order:
1. Exact match: example.com
2. Wildcard match: *.customer-a.com
3. Catch-all: (empty)
```

**Step 2: Check Zone Extraction**

- For domain `shop.customer-a.com`, zone is `customer-a.com`
- For domain `www.example.com`, zone is `example.com`
- For domain `api.sub.example.com`, zone is `example.com` (not `sub.example.com`)

**Step 3: Review All Credential Zone Filters**

```
Settings → DNS Providers → Manage Credentials
List all zone filters and check for overlaps:

Credential A: example.com (exact)
Credential B: *.example.com (wildcard)
Credential C: (empty - catch-all)

For www.example.com:
- Zone: example.com
- Match: Credential A (exact match wins)
```

**Step 4: Adjust Zone Filters**

- Make zone filters more specific to avoid unwanted matches
- Remove or narrow catch-all if it's too broad
- Use exact matches for critical domains

**Step 5: Test Zone Matching**

- Some Charon versions may include a zone matching test utility
- Go to **Settings** → **DNS Providers** → **Test Zone Matching**
- Enter a domain and see which credential would be selected

#### Issue 5: Credential test succeeds but certificate issuance fails

**Symptoms:**

- Credential test passes: ✅ "Credential validated successfully"
- Certificate issuance fails with DNS-related error
- Error: `DNS propagation timeout` or `TXT record not found`

**Root Causes:**

1. API permissions sufficient for test but not for DNS-01 challenge
2. DNS propagation delay
3. Credential has access to different zones than expected
4. Firewall/network issue blocking DNS updates

**Solutions:**

**Step 1: Check API Permissions**

**Cloudflare:**

- Required: `Zone:DNS:Edit` permission
- Test permission alone may only check `Zone:DNS:Read`

**Route53:**

- Required: `route53:ChangeResourceRecordSets`, `route53:GetChange`
- Test permission alone may only check `route53:ListHostedZones`

**Step 2: Verify Zone Access**

- Ensure credential has access to the specific zone
- Check DNS provider dashboard for zone visibility
- For Route53, ensure IAM policy includes the correct hosted zone ID

**Step 3: Check DNS Propagation**

- DNS-01 challenge requires TXT record to propagate
- Default timeout: 60 seconds
- Increase timeout in Charon settings if DNS provider is slow:

  ```
  Settings → Advanced → DNS Propagation Timeout: 120 seconds
  ```

**Step 4: Manual DNS Test**

- After certificate issuance fails, check if TXT record was created:

  ```bash
  dig TXT _acme-challenge.shop.customer-a.com
  nslookup -type=TXT _acme-challenge.shop.customer-a.com
  ```

- If record exists, issue is with propagation delay
- If record doesn't exist, issue is with API permissions or credential

**Step 5: Review Let's Encrypt Logs**

- View detailed certificate issuance logs:

  ```
  Settings → Logs → Filter by: "certificate_issuance"
  ```

- Look for specific error messages from Let's Encrypt or DNS provider

### Getting Help

If you continue to experience issues:

1. **Check Documentation**: Review [DNS provider-specific guides](#) for configuration details
2. **Review Audit Logs**: Check audit trail for clues about what went wrong
3. **Test Credentials**: Use credential test feature to isolate API issues
4. **Enable Debug Logging**: Temporarily enable debug logging for certificate operations
5. **Community Support**: Visit Charon community forums or GitHub discussions
6. **Professional Support**: Contact Charon support team for enterprise customers

## API Reference

### Authentication

All API requests require authentication using a Charon API token:

```bash
curl -H "Authorization: Bearer YOUR_API_TOKEN" \
     https://your-charon-instance/api/v1/...
```

**Getting an API Token:**

1. Go to **Settings** → **API Tokens**
2. Click **Generate Token**
3. Copy token (shown only once)

### Credential Management API Endpoints

#### List Credentials

**Endpoint:** `GET /api/v1/dns-providers/{providerId}/credentials`

**Description:** List all credentials for a DNS provider

**Request:**

```bash
curl -X GET \
  https://your-charon-instance/api/v1/dns-providers/1/credentials \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

**Response:**

```json
{
  "credentials": [
    {
      "id": 42,
      "provider_id": 1,
      "name": "Customer A Production",
      "description": "Credential for Customer A production domains",
      "zone_filter": "*.customer-a.com",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-03T10:45:00Z",
      "last_used_at": "2026-01-03T10:45:00Z",
      "usage_count": 12,
      "success_count": 156,
      "failure_count": 2
    },
    {
      "id": 43,
      "provider_id": 1,
      "name": "Catch-All",
      "description": "Fallback credential",
      "zone_filter": "",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z",
      "last_used_at": "2026-01-02T15:30:00Z",
      "usage_count": 5,
      "success_count": 10,
      "failure_count": 0
    }
  ],
  "total": 2
}
```

#### Get Credential

**Endpoint:** `GET /api/v1/dns-providers/{providerId}/credentials/{credentialId}`

**Description:** Get details of a specific credential

**Request:**

```bash
curl -X GET \
  https://your-charon-instance/api/v1/dns-providers/1/credentials/42 \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

**Response:**

```json
{
  "id": 42,
  "provider_id": 1,
  "name": "Customer A Production",
  "description": "Credential for Customer A production domains",
  "zone_filter": "*.customer-a.com",
  "created_at": "2026-01-01T00:00:00Z",
  "updated_at": "2026-01-03T10:45:00Z",
  "last_used_at": "2026-01-03T10:45:00Z",
  "usage_count": 12,
  "success_count": 156,
  "failure_count": 2,
  "domains": [
    "shop.customer-a.com",
    "api.customer-a.com",
    "portal.customer-a.com"
  ]
}
```

#### Create Credential

**Endpoint:** `POST /api/v1/dns-providers/{providerId}/credentials`

**Description:** Create a new credential for a DNS provider

**Request:**

```bash
curl -X POST \
  https://your-charon-instance/api/v1/dns-providers/1/credentials \
  -H "Authorization: Bearer YOUR_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Customer B Production",
    "description": "Credential for Customer B production domains",
    "zone_filter": "*.customer-b.com",
    "credentials": {
      "api_token": "your-cloudflare-api-token"
    }
  }'
```

**Provider-Specific Credential Fields:**

**Cloudflare:**

```json
"credentials": {
  "api_token": "your-cloudflare-api-token"
}
// OR
"credentials": {
  "api_key": "your-cloudflare-api-key",
  "email": "your-email@example.com"
}
```

**Route53:**

```json
"credentials": {
  "access_key_id": "AKIAIOSFODNN7EXAMPLE",
  "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
}
```

**DigitalOcean:**

```json
"credentials": {
  "api_token": "your-digitalocean-api-token"
}
```

**Response:**

```json
{
  "id": 44,
  "provider_id": 1,
  "name": "Customer B Production",
  "description": "Credential for Customer B production domains",
  "zone_filter": "*.customer-b.com",
  "created_at": "2026-01-04T15:30:00Z",
  "updated_at": "2026-01-04T15:30:00Z",
  "last_used_at": null,
  "usage_count": 0,
  "success_count": 0,
  "failure_count": 0
}
```

#### Update Credential

**Endpoint:** `PATCH /api/v1/dns-providers/{providerId}/credentials/{credentialId}`

**Description:** Update an existing credential

**Request:**

```bash
curl -X PATCH \
  https://your-charon-instance/api/v1/dns-providers/1/credentials/44 \
  -H "Authorization: Bearer YOUR_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Updated credential description",
    "zone_filter": "*.customer-b.com,*.customer-b.net"
  }'
```

**Response:**

```json
{
  "id": 44,
  "provider_id": 1,
  "name": "Customer B Production",
  "description": "Updated credential description",
  "zone_filter": "*.customer-b.com,*.customer-b.net",
  "created_at": "2026-01-04T15:30:00Z",
  "updated_at": "2026-01-04T16:00:00Z",
  "last_used_at": null,
  "usage_count": 0,
  "success_count": 0,
  "failure_count": 0
}
```

#### Delete Credential

**Endpoint:** `DELETE /api/v1/dns-providers/{providerId}/credentials/{credentialId}`

**Description:** Delete a credential (fails if credential is in use)

**Request:**

```bash
curl -X DELETE \
  https://your-charon-instance/api/v1/dns-providers/1/credentials/44 \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

**Response (Success):**

```json
{
  "message": "Credential deleted successfully",
  "id": 44
}
```

**Response (Error - In Use):**

```json
{
  "error": "Cannot delete credential: 3 proxy hosts are using this credential",
  "affected_domains": [
    "shop.customer-b.com",
    "api.customer-b.com",
    "portal.customer-b.com"
  ]
}
```

#### Test Credential

**Endpoint:** `POST /api/v1/dns-providers/{providerId}/credentials/{credentialId}/test`

**Description:** Test if a credential is valid and has correct permissions

**Request:**

```bash
curl -X POST \
  https://your-charon-instance/api/v1/dns-providers/1/credentials/42/test \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

**Response (Success):**

```json
{
  "status": "success",
  "message": "Credential validated successfully",
  "details": {
    "provider": "cloudflare",
    "test_performed": "zone_list",
    "accessible_zones": [
      "customer-a.com"
    ]
  }
}
```

**Response (Failure):**

```json
{
  "status": "failed",
  "message": "API authentication failed",
  "details": {
    "provider": "cloudflare",
    "error_code": "403",
    "error_message": "Invalid credentials"
  }
}
```

#### Enable Multi-Credential Mode

**Endpoint:** `POST /api/v1/dns-providers/{providerId}/enable-multi-credential`

**Description:** Enable multi-credential mode for a DNS provider

**Request:**

```bash
curl -X POST \
  https://your-charon-instance/api/v1/dns-providers/1/enable-multi-credential \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

**Response:**

```json
{
  "message": "Multi-credential mode enabled",
  "provider_id": 1,
  "migrated_credential": {
    "id": 45,
    "name": "Cloudflare Primary Credential",
    "zone_filter": "",
    "description": "Migrated from single-credential mode"
  }
}
```

#### Disable Multi-Credential Mode

**Endpoint:** `POST /api/v1/dns-providers/{providerId}/disable-multi-credential`

**Description:** Disable multi-credential mode (reverts to first credential as primary)

**Request:**

```bash
curl -X POST \
  https://your-charon-instance/api/v1/dns-providers/1/disable-multi-credential \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

**Response:**

```json
{
  "message": "Multi-credential mode disabled",
  "provider_id": 1,
  "primary_credential": {
    "id": 45,
    "name": "Cloudflare Primary Credential"
  },
  "note": "Other credentials are retained but not used. Re-enable multi-credential mode to use them again."
}
```

### Error Responses

All API endpoints may return the following error responses:

**400 Bad Request:**

```json
{
  "error": "Invalid zone filter format",
  "details": "Zone filter cannot contain spaces"
}
```

**401 Unauthorized:**

```json
{
  "error": "Unauthorized",
  "message": "Invalid or expired API token"
}
```

**403 Forbidden:**

```json
{
  "error": "Forbidden",
  "message": "Insufficient permissions to manage credentials"
}
```

**404 Not Found:**

```json
{
  "error": "Not found",
  "message": "Credential with ID 999 not found"
}
```

**409 Conflict:**

```json
{
  "error": "Conflict",
  "message": "Zone filter 'example.com' conflicts with existing credential",
  "conflicting_credential_id": 42
}
```

**500 Internal Server Error:**

```json
{
  "error": "Internal server error",
  "message": "An unexpected error occurred. Please contact support.",
  "request_id": "req_abc123xyz"
}
```

## Cross-References

### Related Documentation

- **[DNS Provider Setup Guides](../dns-providers/)** - Configure individual DNS providers
- **[Audit Logging](../security/audit-logging.md)** - View and export audit logs for credential operations
- **[Security Best Practices](../security/best-practices.md)** - Security guidelines for credential management
- **[Key Rotation](../security/key-rotation.md)** - Automated credential rotation strategies
- **[Certificate Management](../certificates/)** - Understanding Let's Encrypt certificate lifecycle
- **[API Documentation](../api/)** - Complete API reference for automation
- **[Multi-Tenancy Guide](../deployment/multi-tenancy.md)** - Deploying Charon for multi-tenant scenarios
- **[Backup and Recovery](../maintenance/backup-recovery.md)** - Backing up credential configuration

### Provider-Specific Guides

- **[Cloudflare Multi-Credential Setup](../dns-providers/cloudflare-multi-credential.md)**
- **[Route53 IAM Policies for Multi-Credential](../dns-providers/route53-multi-credential.md)**
- **[DigitalOcean Token Scoping](../dns-providers/digitalocean-multi-credential.md)**

### Tutorials

- **[Tutorial: Setting Up Multi-Credential for an MSP](../tutorials/msp-multi-credential.md)**
- **[Tutorial: Environment Separation with Multi-Credentials](../tutorials/environment-separation.md)**
- **[Tutorial: Migrating from Single to Multi-Credential Mode](../tutorials/migration-multi-credential.md)**

---

## Support and Feedback

**Questions?** Visit the [Charon Community Forums](https://community.charon.example.com)

**Found a Bug?** Report it on [GitHub Issues](https://github.com/charon/charon/issues)

**Feature Request?** Submit your ideas in [GitHub Discussions](https://github.com/charon/charon/discussions)

**Need Help?** Contact support at [support@charon.example.com](mailto:support@charon.example.com)

---

_Last Updated: January 4, 2026_
_Version: 1.3.0_
