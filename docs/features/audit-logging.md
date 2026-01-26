# Audit Logging

Charon's audit logging system provides comprehensive tracking of all DNS provider credential operations, giving you complete visibility into who accessed, modified, or used sensitive credentials.

## Overview

Audit logging automatically records security-sensitive operations for compliance, security monitoring, and troubleshooting. Every action involving DNS provider credentials is tracked with full context including:

- **Who**: User ID or system actor
- **What**: Specific action performed (create, update, delete, test, decrypt)
- **When**: Precise timestamp
- **Where**: IP address and user agent
- **Why**: Full event context and metadata

### Why Audit Logging Matters

- **Security Monitoring**: Detect unauthorized access or suspicious patterns
- **Compliance**: Meet SOC 2, GDPR, HIPAA, and PCI-DSS requirements for audit trails
- **Troubleshooting**: Diagnose certificate issuance failures retrospectively
- **Accountability**: Track all credential operations with full attribution

## Accessing Audit Logs

### Navigation

1. Navigate to **Security** in the main menu
2. Click **Audit Logs** in the submenu
3. The audit log table displays recent events with pagination

### UI Overview

The audit log interface consists of:

- **Data Table**: Lists all audit events with key information
- **Filter Bar**: Refine results by date, category, actor, action, or resource
- **Search Box**: Full-text search across event details
- **Details Modal**: View complete event information with related events
- **Export Button**: Download audit logs as CSV for external analysis

## Understanding Audit Events

### Event Categories

All audit events are categorized for easy filtering:

| Category | Description | Example Events |
|----------|-------------|----------------|
| `dns_provider` | DNS provider credential operations | Create, update, delete, test credentials |
| `certificate` | Certificate lifecycle events | Issuance, renewal, failure |
| `system` | System-level operations | Automated credential decryption |

### Event Actions

Charon logs the following DNS provider operations:

| Action | When It's Logged | Details Captured |
|--------|------------------|------------------|
| `dns_provider_create` | New DNS provider added | Provider name, type, is_default flag |
| `dns_provider_update` | Provider settings changed | Changed fields, old values, new values |
| `dns_provider_delete` | Provider removed | Provider name, type, whether credentials existed |
| `credential_test` | Credentials tested via API | Provider name, test result, error message |
| `credential_decrypt` | Caddy reads credentials for cert issuance | Provider name, purpose (certificate_issuance) |
| `certificate_issued` | Certificate successfully issued | Domain, provider used, success/failure status |

## Filtering and Search

### Date Range Filter

Filter events by time period:

1. Click the **Date Range** dropdown
2. Select a preset (**Last 24 Hours**, **Last 7 Days**, **Last 30 Days**, **Last 90 Days**)
3. Or select **Custom Range** and pick specific start and end dates
4. Results update automatically

### Category Filter

Filter by event category:

1. Click the **Category** dropdown
2. Select one or more categories (dns_provider, certificate, system)
3. Only events matching selected categories will be displayed

### Actor Filter

Filter by who performed the action:

1. Click the **Actor** dropdown
2. Select a user from the list (shows both username and user ID)
3. Select **System** to see automated operations
4. View only events from the selected actor

### Action Filter

Filter by specific operation type:

1. Click the **Action** dropdown
2. Select one or more actions (create, update, delete, test, decrypt)
3. Results show only the selected action types

### Resource Filter

Filter by specific DNS provider:

1. Click the **Resource** dropdown
2. Select a DNS provider from the list
3. View only events related to that provider

### Search

Perform free-text search across all event details:

1. Enter search terms in the **Search** box
2. Press Enter or click the search icon
3. Results include events where the search term appears in:
   - Provider name
   - Event details JSON
   - IP addresses
   - User agents

### Clearing Filters

- Click the **Clear Filters** button to reset all filters
- Filters persist while navigating within the audit log page
- Filters reset when you leave and return to the page

## Viewing Event Details

### Opening the Details Modal

1. Click any row in the audit log table
2. Or click the **View Details** button on the right side of a row

### Details Modal Contents

The details modal displays:

- **Event UUID**: Unique identifier for the event
- **Timestamp**: Exact date and time (ISO 8601 format)
- **Actor**: User ID or "system" for automated operations
- **Action**: Operation performed
- **Category**: Event category (dns_provider, certificate, etc.)
- **Resource**: DNS provider name and UUID
- **IP Address**: Client IP that initiated the operation
- **User Agent**: Browser or API client information
- **Full Details**: Complete JSON payload with all event metadata

### Understanding the Details JSON

The details field contains a JSON object with event-specific information:

**Create Event Example:**

```json
{
  "name": "Cloudflare Production",
  "type": "cloudflare",
  "is_default": true
}
```

**Update Event Example:**

```json
{
  "changed_fields": ["credentials", "is_default"],
  "old_values": {
    "is_default": false
  },
  "new_values": {
    "is_default": true
  }
}
```

**Test Event Example:**

```json
{
  "test_result": "success",
  "response_time_ms": 342
}
```

**Decrypt Event Example:**

```json
{
  "purpose": "certificate_issuance",
  "success": true
}
```

### Finding Related Events

1. In the details modal, note the **Resource UUID**
2. Click **View Related Events** to see all events for this resource
3. Or manually filter by Resource UUID using the filter bar

## Exporting Audit Logs

### CSV Export

Export audit logs for external analysis, compliance reporting, or archival:

1. Apply desired filters to narrow down events
2. Click the **Export CSV** button
3. A CSV file downloads with the following columns:
   - Timestamp
   - Actor
   - Action
   - Event Category
   - Resource ID
   - Resource UUID
   - IP Address
   - User Agent
   - Details

### Export Use Cases

- **Compliance Reports**: Generate quarterly audit reports for SOC 2
- **Security Analysis**: Import into SIEM tools for threat detection
- **Forensics**: Investigate security incidents with complete audit trail
- **Backup**: Archive audit logs beyond the retention period

### Export Limitations

- Exports are limited to 10,000 events per download
- For larger exports, use date range filters to split into multiple files
- Exports respect all active filters (date, category, actor, etc.)

## Event Scenarios

### Scenario 1: New DNS Provider Setup

**Timeline:**

1. User `admin@example.com` logs in from `192.168.1.100`
2. Navigates to DNS Providers page
3. Clicks "Add DNS Provider"
4. Fills in Cloudflare credentials and clicks Save

**Audit Log Entries:**

```
2026-01-03 14:23:45 | user:5 | dns_provider_create | dns_provider | {"name":"Cloudflare Prod","type":"cloudflare","is_default":true}
```

### Scenario 2: Credential Testing

**Timeline:**

1. User tests existing provider credentials
2. API validation succeeds

**Audit Log Entries:**

```
2026-01-03 14:25:12 | user:5 | credential_test | dns_provider | {"test_result":"success","response_time_ms":342}
```

### Scenario 3: Certificate Issuance

**Timeline:**

1. Caddy detects new host requires SSL certificate
2. Caddy decrypts DNS provider credentials
3. ACME DNS-01 challenge completes successfully
4. Certificate issued

**Audit Log Entries:**

```
2026-01-03 14:30:00 | system | credential_decrypt | dns_provider | {"purpose":"certificate_issuance","success":true}
2026-01-03 14:30:45 | system | certificate_issued | certificate | {"domain":"app.example.com","provider":"cloudflare","result":"success"}
```

### Scenario 4: Provider Update

**Timeline:**

1. User updates default provider setting
2. API saves changes

**Audit Log Entries:**

```
2026-01-03 15:00:22 | user:5 | dns_provider_update | dns_provider | {"changed_fields":["is_default"],"old_values":{"is_default":false},"new_values":{"is_default":true}}
```

### Scenario 5: Provider Deletion

**Timeline:**

1. User deletes unused DNS provider
2. Credentials are securely wiped

**Audit Log Entries:**

```
2026-01-03 16:45:33 | user:5 | dns_provider_delete | dns_provider | {"name":"Old Provider","type":"route53","had_credentials":true}
```

## Viewing Provider-Specific Audit History

### From DNS Provider Page

1. Navigate to **Settings** → **DNS Providers**
2. Click on any DNS provider to open the edit form
3. Click the **View Audit History** button
4. See all audit events for this specific provider

### API Endpoint

You can also retrieve provider-specific audit logs via API:

```bash
GET /api/v1/dns-providers/:id/audit-logs?page=1&limit=50
```

## Troubleshooting

### Common Questions

**Q: Why don't I see audit logs from before today?**

A: Audit logging was introduced in Charon v1.2.0. Only events after the feature was enabled are logged. Previous operations are not retroactively logged.

**Q: How long are audit logs kept?**

A: By default, audit logs are retained for 90 days. After 90 days, logs are automatically deleted to prevent unbounded database growth. Administrators can configure the retention period via environment variable `AUDIT_LOG_RETENTION_DAYS`.

**Q: Can audit logs be modified or deleted?**

A: No. Audit logs are immutable and append-only. Only the automatic cleanup job (based on retention policy) can delete logs. This ensures audit trail integrity for compliance purposes.

**Q: What happens if audit logging fails?**

A: Audit logging is non-blocking and asynchronous. If the audit log channel is full or the database is temporarily unavailable, the event is dropped but the primary operation (e.g., creating a DNS provider) succeeds. Dropped events are logged to the application log for monitoring.

**Q: Do audit logs include credential values?**

A: No. Audit logs never include actual credential values (API keys, tokens, passwords). Only metadata about the operation is logged (provider name, type, whether credentials were present).

**Q: Can I see who viewed credentials?**

A: Credentials are never "viewed" directly. The only access logged is when credentials are decrypted for certificate issuance (logged as `credential_decrypt` with actor "system").

### Performance Impact

Audit logging is designed for minimal performance impact:

- **Asynchronous Writes**: Audit events are written via a buffered channel and background goroutine
- **Non-Blocking**: Failed audit writes do not block API operations
- **Indexed Queries**: Database indexes on `created_at`, `event_category`, `resource_uuid`, and `actor` ensure fast filtering
- **Automatic Cleanup**: Old logs are periodically deleted to prevent database bloat

**Typical Impact:**

- API request latency: +0.1ms (sending to channel)
- Database writes: Batched in background, no user-facing impact
- Storage: ~500 bytes per event, ~1.5 GB per year at 100 events/day

### Missing Events

If you expect to see an event but don't:

1. **Check filters**: Clear all filters and search to see all events
2. **Check date range**: Expand date range to "Last 90 Days"
3. **Check retention policy**: Event may have been automatically deleted
4. **Check application logs**: Look for "audit channel full" or "Failed to write audit log" messages

### Slow Query Performance

If audit log pages load slowly:

1. **Narrow date range**: Searching 90 days of logs is slower than 7 days
2. **Use specific filters**: Filter by category, actor, or action before searching
3. **Check database indexes**: Ensure indexes on `security_audits` table are present
4. **Consider archival**: Export and delete old logs if database is very large

## API Reference

### List Audit Logs

Retrieve audit logs with pagination and filtering.

**Endpoint:**

```http
GET /api/v1/audit-logs
```

**Query Parameters:**

- `page` (int, default: 1): Page number
- `limit` (int, default: 50, max: 100): Results per page
- `actor` (string): Filter by actor (user ID or "system")
- `action` (string): Filter by action type
- `event_category` (string): Filter by category (dns_provider, certificate, etc.)
- `resource_uuid` (string): Filter by resource UUID
- `start_date` (RFC3339): Start of date range
- `end_date` (RFC3339): End of date range

**Example Request:**

```bash
curl -X GET "https://charon.example.com/api/v1/audit-logs?page=1&limit=50&event_category=dns_provider&start_date=2026-01-01T00:00:00Z" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Response:**

```json
{
  "audit_logs": [
    {
      "id": 1,
      "uuid": "550e8400-e29b-41d4-a716-446655440000",
      "actor": "user:5",
      "action": "dns_provider_create",
      "event_category": "dns_provider",
      "resource_id": 3,
      "resource_uuid": "660e8400-e29b-41d4-a716-446655440001",
      "details": "{\"name\":\"Cloudflare\",\"type\":\"cloudflare\",\"is_default\":true}",
      "ip_address": "192.168.1.100",
      "user_agent": "Mozilla/5.0 (X11; Linux x86_64) Chrome/120.0",
      "created_at": "2026-01-03T14:23:45Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 1,
    "total_pages": 1
  }
}
```

### Get Single Audit Event

Retrieve complete details for a specific audit event.

**Endpoint:**

```http
GET /api/v1/audit-logs/:uuid
```

**Parameters:**

- `uuid` (string, required): Event UUID

**Example Request:**

```bash
curl -X GET "https://charon.example.com/api/v1/audit-logs/550e8400-e29b-41d4-a716-446655440000" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Response:**

```json
{
  "id": 1,
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "actor": "user:5",
  "action": "dns_provider_create",
  "event_category": "dns_provider",
  "resource_id": 3,
  "resource_uuid": "660e8400-e29b-41d4-a716-446655440001",
  "details": "{\"name\":\"Cloudflare\",\"type\":\"cloudflare\",\"is_default\":true}",
  "ip_address": "192.168.1.100",
  "user_agent": "Mozilla/5.0 (X11; Linux x86_64) Chrome/120.0",
  "created_at": "2026-01-03T14:23:45Z"
}
```

### Get Provider Audit History

Retrieve all audit events for a specific DNS provider.

**Endpoint:**

```http
GET /api/v1/dns-providers/:id/audit-logs
```

**Parameters:**

- `id` (int, required): DNS provider ID

**Query Parameters:**

- `page` (int, default: 1): Page number
- `limit` (int, default: 50, max: 100): Results per page

**Example Request:**

```bash
curl -X GET "https://charon.example.com/api/v1/dns-providers/3/audit-logs?page=1&limit=50" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Response:**

```json
{
  "audit_logs": [
    {
      "id": 3,
      "uuid": "770e8400-e29b-41d4-a716-446655440002",
      "actor": "user:5",
      "action": "dns_provider_update",
      "event_category": "dns_provider",
      "resource_id": 3,
      "resource_uuid": "660e8400-e29b-41d4-a716-446655440001",
      "details": "{\"changed_fields\":[\"is_default\"],\"new_values\":{\"is_default\":true}}",
      "ip_address": "192.168.1.100",
      "user_agent": "Mozilla/5.0 (X11; Linux x86_64) Chrome/120.0",
      "created_at": "2026-01-03T15:00:22Z"
    },
    {
      "id": 1,
      "uuid": "550e8400-e29b-41d4-a716-446655440000",
      "actor": "user:5",
      "action": "dns_provider_create",
      "event_category": "dns_provider",
      "resource_id": 3,
      "resource_uuid": "660e8400-e29b-41d4-a716-446655440001",
      "details": "{\"name\":\"Cloudflare\",\"type\":\"cloudflare\",\"is_default\":true}",
      "ip_address": "192.168.1.100",
      "user_agent": "Mozilla/5.0 (X11; Linux x86_64) Chrome/120.0",
      "created_at": "2026-01-03T14:23:45Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 2,
    "total_pages": 1
  }
}
```

### Authentication

All audit log API endpoints require authentication. Include a valid session cookie or Bearer token:

```bash
# Cookie-based auth (from browser)
Cookie: session=YOUR_SESSION_TOKEN

# Bearer token auth (from API client)
Authorization: Bearer YOUR_API_TOKEN
```

### Error Responses

| Status Code | Error | Description |
|-------------|-------|-------------|
| 400 | Invalid parameter | Invalid page/limit or malformed date |
| 401 | Unauthorized | Missing or invalid authentication |
| 404 | Not found | Audit event UUID does not exist |
| 500 | Server error | Database error or service unavailable |

## Configuration

### Retention Period

Configure how long audit logs are retained before automatic deletion:

**Environment Variable:**

```bash
AUDIT_LOG_RETENTION_DAYS=90  # Default: 90 days
```

**Docker Compose:**

```yaml
services:
  charon:
    environment:
      - AUDIT_LOG_RETENTION_DAYS=180  # 6 months
```

### Channel Buffer Size

Configure the size of the audit log channel buffer (advanced):

**Environment Variable:**

```bash
AUDIT_LOG_CHANNEL_SIZE=1000  # Default: 1000 events
```

Increase if you see "audit channel full" errors in application logs during high-load periods.

## Best Practices

1. **Regular Reviews**: Schedule weekly or monthly reviews of audit logs to spot anomalies
2. **Alert on Patterns**: Set up alerts for suspicious patterns (e.g., bulk deletions, off-hours access)
3. **Export for Compliance**: Regularly export logs for compliance archival before they're auto-deleted
4. **Filter Before Export**: Use filters to export only relevant events for specific audits
5. **Document Procedures**: Create runbooks for investigating common security scenarios
6. **Integrate with SIEM**: Export logs to your SIEM tool for centralized security monitoring
7. **Test Retention Policy**: Verify the retention period meets your compliance requirements

## Security Considerations

- **Immutable Logs**: Audit logs cannot be modified or deleted by users (only auto-cleanup)
- **No Credential Leakage**: Actual credential values are never logged
- **Complete Attribution**: Every event includes actor, IP, and user agent for full traceability
- **Secure Storage**: Audit logs are stored in the same encrypted database as other sensitive data
- **Access Control**: Audit log viewing requires authentication (no anonymous access)

## Related Features

- [DNS Challenge Support](./dns-challenge.md) - Configure DNS providers for automated certificates
- [Security Features](./security.md) - WAF, access control, and security notifications
- [Notifications](./notifications.md) - Get alerts for security events

## Support

For questions or issues with audit logging:

1. Check the [Troubleshooting](#troubleshooting) section above
2. Review the [GitHub Issues](https://github.com/Wikid82/charon/issues) for known problems
3. Open a new issue with the `audit-logging` label
4. Join the [Discord community](https://discord.gg/charon) for real-time support

---

**Last Updated:** January 3, 2026
**Feature Version:** v1.2.0
**Documentation Version:** 1.0
