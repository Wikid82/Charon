````markdown
# Google Cloud DNS Provider Setup

## Overview

Google Cloud DNS is a high-performance, scalable DNS service built on Google's global infrastructure. This guide covers setting up Google Cloud DNS as a provider in Charon for wildcard certificate management.

## Prerequisites

- Google Cloud Platform (GCP) account
- GCP project with billing enabled
- Cloud DNS API enabled
- DNS zone created in Cloud DNS
- Domain nameservers pointing to Google Cloud DNS

## Step 1: Enable Cloud DNS API

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Select your project (or create a new one)
3. Navigate to **APIs & Services** → **Library**
4. Search for **Cloud DNS API**
5. Click **Enable**

> **Note:** The API may take a few minutes to activate after enabling.

## Step 2: Create a Service Account

Create a dedicated service account for Charon with minimal permissions:

1. Navigate to **IAM & Admin** → **Service Accounts**
2. Click **Create Service Account**
3. Configure the service account:
   - **Service account name:** `charon-dns-challenge`
   - **Service account ID:** `charon-dns-challenge` (auto-filled)
   - **Description:** `Service account for Charon DNS-01 ACME challenges`
4. Click **Create and Continue**

## Step 3: Assign DNS Admin Role

1. In the **Grant this service account access to project** section:
   - Click **Select a role**
   - Search for **DNS Administrator**
   - Select **DNS Administrator** (`roles/dns.admin`)
2. Click **Continue**
3. Skip the optional **Grant users access** section
4. Click **Done**

> **Security Note:** For production environments, consider creating a custom role with only the specific permissions needed:
> - `dns.changes.create`
> - `dns.changes.get`
> - `dns.managedZones.list`
> - `dns.resourceRecordSets.create`
> - `dns.resourceRecordSets.delete`
> - `dns.resourceRecordSets.list`
> - `dns.resourceRecordSets.update`

## Step 4: Generate Service Account Key

1. Click on the newly created service account
2. Navigate to the **Keys** tab
3. Click **Add Key** → **Create new key**
4. Select **JSON** format
5. Click **Create**
6. **Save the downloaded JSON file securely** (shown only once)

> **Warning:** The JSON key file contains sensitive credentials. Store it in a password manager or secure vault. Never commit it to version control.

### Example JSON Key Structure

```json
{
  "type": "service_account",
  "project_id": "your-project-id",
  "private_key_id": "key-id",
  "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
  "client_email": "charon-dns-challenge@your-project-id.iam.gserviceaccount.com",
  "client_id": "123456789012345678901",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/..."
}
```

## Step 5: Verify DNS Zone Configuration

Ensure your domain is properly configured in Cloud DNS:

1. Navigate to **Network services** → **Cloud DNS**
2. Verify your zone is listed and active
3. Note the **Zone name** (not the DNS name)
4. Confirm nameservers are correctly assigned:
   - `ns-cloud-a1.googledomains.com`
   - `ns-cloud-a2.googledomains.com`
   - `ns-cloud-a3.googledomains.com`
   - `ns-cloud-a4.googledomains.com`

> **Important:** Update your domain registrar to use Google Cloud DNS nameservers if not already configured.

## Step 6: Configure in Charon

1. Navigate to **DNS Providers** in Charon
2. Click **Add Provider**
3. Fill in the form:
   - **Provider Type:** Select `Google Cloud DNS`
   - **Name:** Enter a descriptive name (e.g., "GCP Cloud DNS - Production")
   - **Project ID:** Enter your GCP project ID (e.g., `my-project-123456`)
   - **Service Account JSON:** Paste the entire contents of the downloaded JSON key file

### Advanced Settings (Optional)

Expand **Advanced Settings** to customize:

- **Propagation Timeout:** `120` seconds (Cloud DNS propagation is typically fast)
- **Polling Interval:** `10` seconds (default)
- **Set as Default:** Enable if this is your primary DNS provider

## Step 7: Test Connection

1. Click **Test Connection** button
2. Wait for validation (usually 5-10 seconds)
3. Verify you see: ✅ **Connection successful**

The test verifies:
- Service account credentials are valid
- Project ID matches the credentials
- Service account has required permissions
- Cloud DNS API is accessible

If the test fails, see [Troubleshooting](#troubleshooting) below.

## Step 8: Save Configuration

Click **Save** to store the DNS provider configuration. Credentials are encrypted at rest using AES-256-GCM.

## Step 9: Use with Wildcard Certificates

When creating a proxy host with a wildcard domain:

1. Navigate to **Proxy Hosts** → **Add Proxy Host**
2. Enter a wildcard domain: `*.example.com`
3. Select **Google Cloud DNS** from the DNS Provider dropdown
4. Configure remaining settings
5. Save

Charon will automatically obtain a wildcard certificate using DNS-01 challenge.

## Example Configuration

```yaml
Provider Type: googleclouddns
Name: GCP Cloud DNS - example.com
Project ID: my-project-123456
Service Account JSON: {"type":"service_account",...}
Propagation Timeout: 120 seconds
Polling Interval: 10 seconds
Default: Yes
```

## Required Permissions

The service account needs the following Cloud DNS permissions:

| Permission | Purpose |
|------------|---------|
| `dns.changes.create` | Create DNS record changes |
| `dns.changes.get` | Check status of DNS changes |
| `dns.managedZones.list` | List available DNS zones |
| `dns.resourceRecordSets.create` | Create TXT records for ACME challenges |
| `dns.resourceRecordSets.delete` | Clean up TXT records after validation |
| `dns.resourceRecordSets.list` | List existing DNS records |
| `dns.resourceRecordSets.update` | Update DNS records if needed |

> **Note:** The **DNS Administrator** role includes all these permissions. For fine-grained control, create a custom role.

## Troubleshooting

### Connection Test Fails

**Error:** `Invalid service account JSON`

- Verify the entire JSON content was pasted correctly
- Ensure no extra whitespace or line breaks were added
- Check the JSON is valid (use a JSON validator)
- Re-download the key file and try again

**Error:** `Project not found` or `Project mismatch`

- Verify the Project ID matches the project in the service account JSON
- Check the `project_id` field in the JSON matches your input
- Ensure the project exists and is active

**Error:** `Permission denied` or `Forbidden`

- Verify the service account has the DNS Administrator role
- Check the role is assigned at the project level
- Ensure Cloud DNS API is enabled
- Wait a few minutes after role assignment (propagation delay)

**Error:** `API not enabled`

- Navigate to APIs & Services → Library
- Search for and enable Cloud DNS API
- Wait 2-3 minutes for activation

### Certificate Issuance Fails

**Error:** `DNS propagation timeout`

- Cloud DNS typically propagates in 30-60 seconds
- Increase Propagation Timeout to 180 seconds
- Verify nameservers are correctly configured with your registrar
- Check Google Cloud Status page for service issues

**Error:** `Zone not found`

- Ensure the DNS zone exists in Cloud DNS
- Verify the domain matches the zone's DNS name
- Check the service account has access to the zone

**Error:** `Record creation failed`

- Check for existing `_acme-challenge` TXT records that may conflict
- Verify service account permissions
- Review Charon logs for detailed API errors

### Nameserver Propagation

**Issue:** DNS changes not visible globally

- Nameserver changes can take 24-48 hours to propagate globally
- Use [DNS Checker](https://dnschecker.org/) to verify propagation
- Verify your registrar shows Google Cloud DNS nameservers
- Wait for full propagation before attempting certificate issuance

### Rate Limiting

Google Cloud DNS API quotas:

- 10,000 queries per day (default)
- 1,000 changes per day (default)
- Certificate challenges typically use <20 requests

If you hit limits:

- Request quota increase via Google Cloud Console
- Reduce frequency of certificate renewals
- Contact Google Cloud support for production workloads

## Security Recommendations

1. **Dedicated Service Account:** Create a separate service account for Charon
2. **Least Privilege:** Use a custom role with only required permissions
3. **Key Rotation:** Rotate service account keys every 90 days
4. **Key Security:** Store JSON key in a secrets manager, never in version control
5. **Audit Logging:** Enable Cloud Audit Logs for DNS API calls
6. **VPC Service Controls:** Consider using VPC Service Controls for additional security
7. **Disable Unused Keys:** Delete old keys immediately after rotation

## Service Account Key Rotation

To rotate the service account key:

1. Create a new key following Step 4
2. Update the configuration in Charon with the new JSON
3. Test the connection to verify the new key works
4. Delete the old key from the GCP console

```bash
# Using gcloud CLI (optional)
# List existing keys
gcloud iam service-accounts keys list \
  --iam-account=charon-dns-challenge@PROJECT_ID.iam.gserviceaccount.com

# Create new key
gcloud iam service-accounts keys create new-key.json \
  --iam-account=charon-dns-challenge@PROJECT_ID.iam.gserviceaccount.com

# Delete old key (after updating Charon)
gcloud iam service-accounts keys delete KEY_ID \
  --iam-account=charon-dns-challenge@PROJECT_ID.iam.gserviceaccount.com
```

## gcloud CLI Verification (Optional)

Test credentials before adding to Charon:

```bash
# Activate service account
gcloud auth activate-service-account \
  --key-file=/path/to/service-account-key.json

# Set project
gcloud config set project YOUR_PROJECT_ID

# List DNS zones
gcloud dns managed-zones list

# Test record creation (creates and deletes a test TXT record)
gcloud dns record-sets create test-acme-challenge.example.com. \
  --zone=your-zone-name \
  --type=TXT \
  --ttl=60 \
  --rrdatas='"test-value"'

# Clean up test record
gcloud dns record-sets delete test-acme-challenge.example.com. \
  --zone=your-zone-name \
  --type=TXT
```

## Additional Resources

- [Google Cloud DNS Documentation](https://cloud.google.com/dns/docs)
- [Service Account Documentation](https://cloud.google.com/iam/docs/service-accounts)
- [Cloud DNS API Reference](https://cloud.google.com/dns/docs/reference/v1)
- [Caddy Google Cloud DNS Module](https://caddyserver.com/docs/modules/dns.providers.googleclouddns)
- [Google Cloud Status Page](https://status.cloud.google.com/)
- [IAM Roles for Cloud DNS](https://cloud.google.com/dns/docs/access-control)

## Related Documentation

- [DNS Providers Overview](../dns-providers.md)
- [Wildcard Certificates Guide](../certificates.md#wildcard-certificates)
- [DNS Challenges Troubleshooting](../../troubleshooting/dns-challenges.md)

````
