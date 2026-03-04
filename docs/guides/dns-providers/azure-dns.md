````markdown
# Azure DNS Provider Setup

## Overview

Azure DNS is Microsoft's cloud-based DNS hosting service that provides name resolution using Microsoft Azure infrastructure. This guide covers setting up Azure DNS as a provider in Charon for wildcard certificate management.

## Prerequisites

- Azure subscription (pay-as-you-go or Enterprise Agreement)
- Azure DNS zone created for your domain
- Domain nameservers pointing to Azure DNS
- Permissions to create App registrations in Microsoft Entra ID (Azure AD)
- Permissions to assign roles in Azure RBAC

## Step 1: Gather Azure Subscription Information

1. Log in to the [Azure Portal](https://portal.azure.com/)
2. Navigate to **Subscriptions**
3. Note your **Subscription ID** (e.g., `12345678-1234-1234-1234-123456789abc`)
4. Navigate to **Resource groups**
5. Note the **Resource group name** containing your DNS zone

> **Tip:** You can find this information in the DNS zone overview page as well.

## Step 2: Verify DNS Zone Configuration

Ensure your domain is properly configured in Azure DNS:

1. Navigate to **DNS zones**
2. Select your DNS zone
3. Note the **Azure nameservers** listed (typically 4 servers like `ns1-01.azure-dns.com`)
4. Verify your domain registrar is configured to use these nameservers

<!-- Screenshot placeholder: Azure DNS zone overview showing nameservers -->

## Step 3: Create App Registration in Microsoft Entra ID

Create an application identity for Charon:

1. Navigate to **Microsoft Entra ID** (formerly Azure Active Directory)
2. Select **App registrations** from the left menu
3. Click **New registration**
4. Configure the application:
   - **Name:** `charon-dns-challenge`
   - **Supported account types:** Select **Accounts in this organizational directory only**
   - **Redirect URI:** Leave blank (not needed for service-to-service auth)
5. Click **Register**

### Note Application Details

After registration, note the following from the **Overview** page:

- **Application (client) ID:** `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
- **Directory (tenant) ID:** `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

<!-- Screenshot placeholder: App registration overview showing client and tenant IDs -->

## Step 4: Create Client Secret

1. In your app registration, navigate to **Certificates & secrets**
2. Click **New client secret**
3. Configure the secret:
   - **Description:** `Charon DNS Challenge`
   - **Expires:** Choose an expiration period (recommended: 12 months or 24 months)
4. Click **Add**
5. **Copy the secret value immediately** (shown only once)

> **Warning:** The client secret value is displayed only once. Copy it now and store it securely. If you lose it, you'll need to create a new secret.

### Secret Expiration Management

| Expiration | Use Case |
|------------|----------|
| 6 months | Development/testing environments |
| 12 months | Production with regular rotation schedule |
| 24 months | Production with less frequent rotation |
| Custom | Enterprise requirements |

## Step 5: Assign DNS Zone Contributor Role

Grant the app registration permission to manage DNS records:

1. Navigate to your **DNS zone**
2. Select **Access control (IAM)** from the left menu
3. Click **Add** → **Add role assignment**
4. In the **Role** tab:
   - Search for **DNS Zone Contributor**
   - Select **DNS Zone Contributor**
   - Click **Next**
5. In the **Members** tab:
   - Select **User, group, or service principal**
   - Click **Select members**
   - Search for `charon-dns-challenge`
   - Select the app registration
   - Click **Select**
6. Click **Review + assign**
7. Click **Review + assign** again to confirm

> **Note:** Role assignments may take a few minutes to propagate.

### Required Permissions

The **DNS Zone Contributor** role includes:

| Permission | Purpose |
|------------|---------|
| `Microsoft.Network/dnsZones/read` | Read DNS zone configuration |
| `Microsoft.Network/dnsZones/TXT/read` | Read TXT records |
| `Microsoft.Network/dnsZones/TXT/write` | Create/update TXT records |
| `Microsoft.Network/dnsZones/TXT/delete` | Delete TXT records |
| `Microsoft.Network/dnsZones/recordsets/read` | List DNS record sets |

> **Security Note:** For tighter security, you can create a custom role with only the permissions listed above.

## Step 6: Configure in Charon

1. Navigate to **DNS Providers** in Charon
2. Click **Add Provider**
3. Fill in the form:
   - **Provider Type:** Select `Azure DNS`
   - **Name:** Enter a descriptive name (e.g., "Azure DNS - Production")
   - **Tenant ID:** Paste the Directory (tenant) ID from Step 3
   - **Client ID:** Paste the Application (client) ID from Step 3
   - **Client Secret:** Paste the secret value from Step 4
   - **Subscription ID:** Paste the Subscription ID from Step 1
   - **Resource Group:** Enter the resource group name containing your DNS zone

### Configuration Fields Summary

| Field | Description | Example |
|-------|-------------|---------|
| **Tenant ID** | Microsoft Entra ID tenant identifier | `12345678-1234-5678-9abc-123456789abc` |
| **Client ID** | App registration application ID | `abcdef12-3456-7890-abcd-ef1234567890` |
| **Client Secret** | App registration secret value | `abc123~XYZ...` |
| **Subscription ID** | Azure subscription identifier | `98765432-1234-5678-9abc-987654321abc` |
| **Resource Group** | Resource group containing DNS zone | `rg-dns-production` |

### Advanced Settings (Optional)

Expand **Advanced Settings** to customize:

- **Propagation Timeout:** `120` seconds (Azure DNS propagates quickly)
- **Polling Interval:** `10` seconds (default)
- **Set as Default:** Enable if this is your primary DNS provider

## Step 7: Test Connection

1. Click **Test Connection** button
2. Wait for validation (usually 5-10 seconds)
3. Verify you see: ✅ **Connection successful**

The test verifies:
- Credentials are valid
- App registration has required permissions
- DNS zone is accessible
- Azure DNS API is reachable

If the test fails, see [Troubleshooting](#troubleshooting) below.

## Step 8: Save Configuration

Click **Save** to store the DNS provider configuration. All credentials are encrypted at rest using AES-256-GCM.

## Step 9: Use with Wildcard Certificates

When creating a proxy host with a wildcard domain:

1. Navigate to **Proxy Hosts** → **Add Proxy Host**
2. Enter a wildcard domain: `*.example.com`
3. Select **Azure DNS** from the DNS Provider dropdown
4. Configure remaining settings
5. Save

Charon will automatically obtain a wildcard certificate using DNS-01 challenge.

## Example Configuration

```yaml
Provider Type: azure
Name: Azure DNS - example.com
Tenant ID: 12345678-1234-5678-9abc-123456789abc
Client ID: abcdef12-3456-7890-abcd-ef1234567890
Client Secret: ****************************************
Subscription ID: 98765432-1234-5678-9abc-987654321abc
Resource Group: rg-dns-production
Propagation Timeout: 120 seconds
Polling Interval: 10 seconds
Default: Yes
```

## Troubleshooting

### Connection Test Fails

**Error:** `Invalid credentials` or `AADSTS7000215: Invalid client secret`

- Verify the client secret was copied correctly
- Check the secret hasn't expired
- Ensure no extra whitespace was added
- Create a new client secret if necessary

**Error:** `AADSTS700016: Application not found`

- Verify the Client ID is correct
- Ensure the app registration exists in the correct tenant
- Check the Tenant ID matches your organization

**Error:** `AADSTS90002: Tenant not found`

- Verify the Tenant ID is correct
- Ensure you're using the correct Azure environment (public vs. government)

**Error:** `Authorization failed` or `Forbidden`

- Verify the DNS Zone Contributor role is assigned
- Check the role is assigned at the DNS zone level
- Wait a few minutes for role assignment propagation
- Verify the resource group name is correct

**Error:** `Resource group not found`

- Check the resource group name spelling (case-sensitive)
- Ensure the resource group exists in the specified subscription
- Verify the subscription ID is correct

**Error:** `DNS zone not found`

- Verify the DNS zone exists in the resource group
- Check the domain matches the DNS zone name
- Ensure the app has access to the subscription

### Certificate Issuance Fails

**Error:** `DNS propagation timeout`

- Azure DNS typically propagates in 30-60 seconds
- Increase Propagation Timeout to 180 seconds
- Verify nameservers are correctly configured with your registrar
- Check Azure Status page for service issues

**Error:** `Record creation failed`

- Verify app registration has DNS Zone Contributor role
- Check for existing `_acme-challenge` TXT records that may conflict
- Review Charon logs for detailed API errors

**Error:** `Rate limit exceeded`

- Azure DNS has API rate limits per subscription
- Increase Polling Interval to reduce API calls
- Contact Azure support to increase limits if needed

### Nameserver Propagation

**Issue:** DNS changes not visible globally

- Nameserver changes can take 24-48 hours to propagate
- Use [DNS Checker](https://dnschecker.org/) to verify global propagation
- Verify your registrar shows Azure DNS nameservers
- Wait for full propagation before attempting certificate issuance

### Client Secret Expiration

**Issue:** Certificates stop renewing

- Client secrets have expiration dates
- Set calendar reminders before expiration
- Create new secret and update Charon configuration before expiry
- Consider using Managed Identities for Azure-hosted Charon deployments

## Security Recommendations

1. **Dedicated App Registration:** Create a separate app registration for Charon
2. **Least Privilege:** Use DNS Zone Contributor role (not broader roles)
3. **Secret Rotation:** Rotate client secrets before expiration (every 6-12 months)
4. **Conditional Access:** Consider conditional access policies for the app
5. **Audit Logging:** Enable Azure Activity Log for DNS operations
6. **Private Endpoints:** Use private endpoints if Charon runs in Azure
7. **Managed Identity:** Use Managed Identity if Charon is hosted in Azure (eliminates secrets)
8. **Monitor Sign-ins:** Review app sign-in logs in Microsoft Entra ID

## Client Secret Rotation

To rotate the client secret:

1. Navigate to your app registration → **Certificates & secrets**
2. Create a new client secret
3. Update the configuration in Charon with the new secret
4. Test the connection to verify the new secret works
5. Delete the old secret from the Azure portal

> **Best Practice:** Create the new secret before the old one expires to avoid downtime.

## Using Azure CLI for Verification (Optional)

Test configuration before adding to Charon:

```bash
# Login with service principal
az login --service-principal \
  --username CLIENT_ID \
  --password CLIENT_SECRET \
  --tenant TENANT_ID

# Set subscription
az account set --subscription SUBSCRIPTION_ID

# List DNS zones
az network dns zone list \
  --resource-group RESOURCE_GROUP_NAME

# Test record creation
az network dns record-set txt add-record \
  --resource-group RESOURCE_GROUP_NAME \
  --zone-name example.com \
  --record-set-name _acme-challenge-test \
  --value "test-value"

# Clean up test record
az network dns record-set txt remove-record \
  --resource-group RESOURCE_GROUP_NAME \
  --zone-name example.com \
  --record-set-name _acme-challenge-test \
  --value "test-value"
```

## Using Managed Identity (Azure-Hosted Charon)

If Charon runs in Azure (VM, Container Instance, AKS), consider using Managed Identity:

1. Enable System-assigned managed identity on your Azure resource
2. Assign **DNS Zone Contributor** role to the managed identity
3. Configure Charon to use managed identity authentication (no secrets needed)

> **Benefits:** No client secrets to manage, automatic credential rotation, enhanced security.

## Azure DNS Limitations

- **Zone-scoped permissions only:** Cannot restrict to specific record types within a zone
- **No private DNS support:** Charon requires public DNS for ACME challenges
- **Regional availability:** Azure DNS is a global service, no regional selection needed
- **Billing:** Azure DNS charges per zone and per million queries

## Cost Considerations

Azure DNS pricing (approximate):

- **Hosted zones:** ~$0.50/month per zone
- **DNS queries:** ~$0.40 per million queries

Certificate challenges generate minimal queries (<100 per certificate issuance).

## Additional Resources

- [Azure DNS Documentation](https://learn.microsoft.com/en-us/azure/dns/)
- [Microsoft Entra ID App Registration](https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-register-app)
- [Azure RBAC for DNS](https://learn.microsoft.com/en-us/azure/dns/dns-protect-zones-recordsets)
- [Caddy Azure DNS Module](https://caddyserver.com/docs/modules/dns.providers.azure)
- [Azure Status Page](https://status.azure.com/)
- [Azure CLI DNS Commands](https://learn.microsoft.com/en-us/cli/azure/network/dns)

## Related Documentation

- [DNS Providers Overview](../dns-providers.md)
- [Wildcard Certificates Guide](../certificates.md#wildcard-certificates)
- [DNS Challenges Troubleshooting](../../troubleshooting/dns-challenges.md)

````
