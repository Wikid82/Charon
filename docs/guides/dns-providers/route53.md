# AWS Route 53 DNS Provider Setup

## Overview

Amazon Route 53 is AWS's scalable DNS service. This guide covers setting up Route 53 as a DNS provider in Charon for wildcard certificate management.

## Prerequisites

- AWS account with Route 53 access
- Domain hosted in Route 53 (public hosted zone)
- IAM permissions to create users and policies
- AWS CLI (optional, for verification)

## Step 1: Create IAM Policy

Create a custom IAM policy with minimum required permissions:

1. Log in to [AWS Console](https://console.aws.amazon.com/)
2. Navigate to **IAM** → **Policies**
3. Click **Create Policy**
4. Select **JSON** tab
5. Paste the following policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "route53:ListHostedZones",
        "route53:GetChange"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "route53:ChangeResourceRecordSets"
      ],
      "Resource": "arn:aws:route53:::hostedzone/*"
    }
  ]
}
```

1. Click **Next: Tags** (optional tags)
2. Click **Next: Review**
3. **Name:** `CharonRoute53DNSChallenge`
4. **Description:** `Allows Charon to manage DNS TXT records for ACME challenges`
5. Click **Create Policy**

> **Tip:** For production, scope the policy to specific hosted zones by replacing `*` with your zone ID.

## Step 2: Create IAM User

Create a dedicated IAM user for Charon:

1. Navigate to **IAM** → **Users**
2. Click **Add Users**
3. **User name:** `charon-dns`
4. Select **Access key - Programmatic access**
5. Click **Next: Permissions**
6. Select **Attach existing policies directly**
7. Search for and select `CharonRoute53DNSChallenge`
8. Click **Next: Tags** (optional)
9. Click **Next: Review**
10. Click **Create User**
11. **Save the credentials** (shown only once):
    - Access Key ID
    - Secret Access Key

> **Warning:** Download the CSV or copy credentials immediately. AWS won't show the secret again.

## Step 3: Configure in Charon

1. Navigate to **DNS Providers** in Charon
2. Click **Add Provider**
3. Fill in the form:
   - **Provider Type:** Select `AWS Route 53`
   - **Name:** Enter a descriptive name (e.g., "AWS Route 53 - Production")
   - **AWS Access Key ID:** Paste the access key from Step 2
   - **AWS Secret Access Key:** Paste the secret key from Step 2
   - **AWS Region:** (Optional) Specify region (default: `us-east-1`)

### Advanced Settings (Optional)

Expand **Advanced Settings** to customize:

- **Propagation Timeout:** `120` seconds (Route 53 propagation can take 60-120 seconds)
- **Polling Interval:** `10` seconds (default)
- **Set as Default:** Enable if this is your primary DNS provider

## Step 4: Test Connection

1. Click **Test Connection** button
2. Wait for validation (may take 5-10 seconds)
3. Verify you see: ✅ **Connection successful**

The test verifies:

- Credentials are valid
- IAM user has required permissions
- Route 53 hosted zones are accessible

If the test fails, see [Troubleshooting](#troubleshooting) below.

## Step 5: Save Configuration

Click **Save** to store the DNS provider configuration. Credentials are encrypted at rest using AES-256-GCM.

## Step 6: Use with Wildcard Certificates

When creating a proxy host with a wildcard domain:

1. Navigate to **Proxy Hosts** → **Add Proxy Host**
2. Enter a wildcard domain: `*.example.com`
3. Select **AWS Route 53** from the DNS Provider dropdown
4. Configure remaining settings
5. Save

Charon will automatically obtain a wildcard certificate using DNS-01 challenge.

## Example Configuration

```yaml
Provider Type: route53
Name: AWS Route 53 - example.com
Access Key ID: AKIAIOSFODNN7EXAMPLE
Secret Access Key: ****************************************
Region: us-east-1
Propagation Timeout: 120 seconds
Polling Interval: 10 seconds
Default: Yes
```

## Required IAM Permissions

The IAM user needs the following Route 53 permissions:

| Action | Resource | Purpose |
|--------|----------|---------|
| `route53:ListHostedZones` | `*` | List available hosted zones |
| `route53:GetChange` | `*` | Check status of DNS changes |
| `route53:ChangeResourceRecordSets` | `arn:aws:route53:::hostedzone/*` | Create/delete TXT records for challenges |

> **Security Best Practice:** Scope `ChangeResourceRecordSets` to specific hosted zone ARNs:

```json
"Resource": "arn:aws:route53:::hostedzone/Z1234567890ABC"
```

## Troubleshooting

### Connection Test Fails

**Error:** `Invalid credentials`

- Verify Access Key ID and Secret Access Key were copied correctly
- Check IAM user exists and is active
- Ensure no extra spaces or characters in credentials

**Error:** `Access denied`

- Verify IAM policy is attached to the user
- Check policy includes all required permissions
- Review CloudTrail logs for denied API calls

**Error:** `Hosted zone not found`

- Ensure domain has a public hosted zone in Route 53
- Verify hosted zone is in the same AWS account
- Check zone is not private (private zones not supported)

### Certificate Issuance Fails

**Error:** `DNS propagation timeout`

- Route 53 propagation typically takes 60-120 seconds
- Increase Propagation Timeout to 180 seconds
- Verify hosted zone is authoritative for the domain
- Check Route 53 name servers match domain registrar settings

**Error:** `Rate limit exceeded`

- Route 53 has API rate limits (5 requests/second per account)
- Increase Polling Interval to 15-20 seconds
- Avoid concurrent certificate requests
- Contact AWS support to increase limits

### Region Configuration

**Issue:** Specifying the wrong region

- Route 53 is a global service; region typically doesn't matter
- Use `us-east-1` (default) if unsure
- Some endpoints may require specific regions
- Check Charon logs if region-specific errors occur

## Security Recommendations

1. **IAM User:** Create a dedicated user for Charon (don't reuse credentials)
2. **Least Privilege:** Use the minimal policy provided above
3. **Scope to Zones:** Limit policy to specific hosted zones in production
4. **Rotate Keys:** Rotate access keys every 90 days
5. **Monitor Usage:** Enable CloudTrail for API activity auditing
6. **MFA Protection:** Enable MFA on the AWS account (not the IAM user)
7. **Access Advisor:** Review IAM Access Advisor to ensure permissions are used

## AWS CLI Verification (Optional)

Test credentials before adding to Charon:

```bash
# Configure AWS CLI with credentials
aws configure --profile charon-dns

# List hosted zones
aws route53 list-hosted-zones --profile charon-dns

# Verify permissions
aws iam get-user --profile charon-dns
```

## Additional Resources

- [AWS Route 53 Documentation](https://docs.aws.amazon.com/route53/)
- [IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
- [Route 53 API Reference](https://docs.aws.amazon.com/route53/latest/APIReference/)
- [Caddy Route 53 Module](https://caddyserver.com/docs/modules/dns.providers.route53)
- [AWS CloudTrail](https://console.aws.amazon.com/cloudtrail/)

## Related Documentation

- [DNS Providers Overview](../dns-providers.md)
- [Wildcard Certificates Guide](../certificates.md#wildcard-certificates)
- [DNS Challenges Troubleshooting](../../troubleshooting/dns-challenges.md)
