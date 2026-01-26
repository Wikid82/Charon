# Notification System

Charon's notification system keeps you informed about important events in your infrastructure through multiple channels, including Discord, Slack, Gotify, Telegram, and custom webhooks.

## Overview

Notifications can be triggered by various events:

- **SSL Certificate Events**: Issued, renewed, or failed
- **Uptime Monitoring**: Host status changes (up/down)
- **Security Events**: WAF blocks, CrowdSec alerts, ACL violations
- **System Events**: Configuration changes, backup completions

## Supported Services

| Service | JSON Templates | Native API | Rich Formatting |
|---------|----------------|------------|-----------------|
| **Discord** | ✅ Yes | ✅ Webhooks | ✅ Embeds |
| **Slack** | ✅ Yes | ✅ Incoming Webhooks | ✅ Block Kit |
| **Gotify** | ✅ Yes | ✅ REST API | ✅ Extras |
| **Generic Webhook** | ✅ Yes | ✅ HTTP POST | ✅ Custom |
| **Telegram** | ❌ No | ✅ Bot API | ⚠️ Markdown |

### Why JSON Templates?

JSON templates give you complete control over notification formatting, allowing you to:

- **Customize appearance**: Use rich embeds, colors, and formatting
- **Add metadata**: Include custom fields, timestamps, and links
- **Optimize visibility**: Structure messages for better readability
- **Integrate seamlessly**: Match your team's existing notification styles

## Configuration

### Basic Setup

1. Navigate to **Settings** → **Notifications**
2. Click **"Add Provider"**
3. Select your service type
4. Enter the webhook URL
5. Configure notification triggers
6. Save your provider

### JSON Template Support

For services supporting JSON (Discord, Slack, Gotify, Generic, Webhook), you can choose from three template options:

#### 1. Minimal Template (Default)

Simple, clean notifications with essential information:

```json
{
  "content": "{{.Title}}: {{.Message}}"
}
```

**Use when:**

- You want low-noise notifications
- Space is limited (mobile notifications)
- Only essential info is needed

#### 2. Detailed Template

Comprehensive notifications with all available context:

```json
{
  "embeds": [{
    "title": "{{.Title}}",
    "description": "{{.Message}}",
    "color": {{.Color}},
    "timestamp": "{{.Timestamp}}",
    "fields": [
      {"name": "Event Type", "value": "{{.EventType}}", "inline": true},
      {"name": "Host", "value": "{{.HostName}}", "inline": true}
    ]
  }]
}
```

**Use when:**

- You need full event context
- Multiple team members review notifications
- Historical tracking is important

#### 3. Custom Template

Create your own template with complete control over structure and formatting.

**Use when:**

- Standard templates don't meet your needs
- You have specific formatting requirements
- Integrating with custom systems

## Service-Specific Examples

### Discord Webhooks

Discord supports rich embeds with colors, fields, and timestamps.

#### Basic Embed

```json
{
  "embeds": [{
    "title": "{{.Title}}",
    "description": "{{.Message}}",
    "color": {{.Color}},
    "timestamp": "{{.Timestamp}}"
  }]
}
```

#### Advanced Embed with Fields

```json
{
  "username": "Charon Alerts",
  "avatar_url": "https://example.com/charon-icon.png",
  "embeds": [{
    "title": "🚨 {{.Title}}",
    "description": "{{.Message}}",
    "color": {{.Color}},
    "timestamp": "{{.Timestamp}}",
    "fields": [
      {
        "name": "Event Type",
        "value": "{{.EventType}}",
        "inline": true
      },
      {
        "name": "Severity",
        "value": "{{.Severity}}",
        "inline": true
      },
      {
        "name": "Host",
        "value": "{{.HostName}}",
        "inline": false
      }
    ],
    "footer": {
      "text": "Charon Notification System"
    }
  }]
}
```

**Available Discord Colors:**

- `2326507` - Blue (info)
- `15158332` - Red (error)
- `16776960` - Yellow (warning)
- `3066993` - Green (success)

### Slack Webhooks

Slack uses Block Kit for rich message formatting.

#### Basic Block

```json
{
  "text": "{{.Title}}",
  "blocks": [
    {
      "type": "header",
      "text": {
        "type": "plain_text",
        "text": "{{.Title}}"
      }
    },
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "{{.Message}}"
      }
    }
  ]
}
```

#### Advanced Block with Context

```json
{
  "text": "{{.Title}}",
  "blocks": [
    {
      "type": "header",
      "text": {
        "type": "plain_text",
        "text": "🔔 {{.Title}}",
        "emoji": true
      }
    },
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "*Event:* {{.EventType}}\n*Message:* {{.Message}}"
      }
    },
    {
      "type": "section",
      "fields": [
        {
          "type": "mrkdwn",
          "text": "*Host:*\n{{.HostName}}"
        },
        {
          "type": "mrkdwn",
          "text": "*Time:*\n{{.Timestamp}}"
        }
      ]
    },
    {
      "type": "context",
      "elements": [
        {
          "type": "mrkdwn",
          "text": "Notification from Charon"
        }
      ]
    }
  ]
}
```

**Slack Markdown Tips:**

- `*bold*` for emphasis
- `_italic_` for subtle text
- `~strike~` for deprecated info
- `` `code` `` for technical details
- Use `\n` for line breaks

### Gotify Webhooks

Gotify supports JSON payloads with priority levels and extras.

#### Basic Message

```json
{
  "title": "{{.Title}}",
  "message": "{{.Message}}",
  "priority": 5
}
```

#### Advanced Message with Extras

```json
{
  "title": "{{.Title}}",
  "message": "{{.Message}}",
  "priority": {{.Priority}},
  "extras": {
    "client::display": {
      "contentType": "text/markdown"
    },
    "client::notification": {
      "click": {
        "url": "https://your-charon-instance.com"
      }
    },
    "charon": {
      "event_type": "{{.EventType}}",
      "host_name": "{{.HostName}}",
      "timestamp": "{{.Timestamp}}"
    }
  }
}
```

**Gotify Priority Levels:**

- `0` - Very low
- `2` - Low
- `5` - Normal (default)
- `8` - High
- `10` - Very high (emergency)

### Generic Webhooks

For custom integrations, use any JSON structure:

```json
{
  "notification": {
    "type": "{{.EventType}}",
    "level": "{{.Severity}}",
    "title": "{{.Title}}",
    "body": "{{.Message}}",
    "metadata": {
      "host": "{{.HostName}}",
      "timestamp": "{{.Timestamp}}",
      "source": "charon"
    }
  }
}
```

## Template Variables

All services support these variables in JSON templates:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Title}}` | Event title | "SSL Certificate Renewed" |
| `{{.Message}}` | Event message/details | "Certificate for example.com renewed" |
| `{{.EventType}}` | Type of event | "ssl_renewal", "uptime_down" |
| `{{.Severity}}` | Event severity level | "info", "warning", "error" |
| `{{.HostName}}` | Affected proxy host | "example.com" |
| `{{.Timestamp}}` | ISO 8601 timestamp | "2025-12-24T10:30:00Z" |
| `{{.Color}}` | Color code (integer) | 2326507 (blue) |
| `{{.Priority}}` | Numeric priority (1-10) | 5 |

### Event-Specific Variables

Some events include additional variables:

**SSL Certificate Events:**

- `{{.Domain}}` - Certificate domain
- `{{.ExpiryDate}}` - Expiration date
- `{{.DaysRemaining}}` - Days until expiry

**Uptime Events:**

- `{{.StatusChange}}` - "up_to_down" or "down_to_up"
- `{{.ResponseTime}}` - Last response time in ms
- `{{.Downtime}}` - Duration of downtime

**Security Events:**

- `{{.AttackerIP}}` - Source IP address
- `{{.RuleID}}` - Triggered rule identifier
- `{{.Action}}` - Action taken (block/log)

## Migration Guide

### Upgrading from Basic Webhooks

If you've been using webhook providers without JSON templates:

**Before (Basic webhook):**

```
Type: webhook
URL: https://discord.com/api/webhooks/...
Template: (not available)
```

**After (JSON template):**

```
Type: discord
URL: https://discord.com/api/webhooks/...
Template: detailed (or custom)
```

**Steps:**

1. Edit your existing provider
2. Change type from `webhook` to the specific service (e.g., `discord`)
3. Select a template (minimal, detailed, or custom)
4. Test the notification
5. Save changes

### Testing Your Template

Before saving, always test your template:

1. Click **"Send Test Notification"** in the provider form
2. Check your notification channel (Discord/Slack/etc.)
3. Verify formatting, colors, and all fields appear correctly
4. Adjust template if needed
5. Test again until satisfied

## Troubleshooting

### Template Validation Errors

**Error:** `Invalid JSON template`

**Solution:** Validate your JSON using a tool like [jsonlint.com](https://jsonlint.com). Common issues:

- Missing closing braces `}`
- Trailing commas
- Unescaped quotes in strings

**Error:** `Template variable not found: {{.CustomVar}}`

**Solution:** Only use supported template variables listed above.

### Notification Not Received

**Checklist:**

1. ✅ Provider is enabled
2. ✅ Event type is configured for notifications
3. ✅ Webhook URL is correct
4. ✅ Service (Discord/Slack/etc.) is online
5. ✅ Test notification succeeds
6. ✅ Check Charon logs for errors: `docker logs charon | grep notification`

### Discord Embed Not Showing

**Cause:** Embeds require specific structure.

**Solution:** Ensure your template includes the `embeds` array:

```json
{
  "embeds": [
    {
      "title": "{{.Title}}",
      "description": "{{.Message}}"
    }
  ]
}
```

### Slack Message Appears Plain

**Cause:** Block Kit requires specific formatting.

**Solution:** Use `blocks` array with proper types:

```json
{
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "{{.Message}}"
      }
    }
  ]
}
```

## Best Practices

### 1. Start Simple

Begin with the **minimal** template and only customize if you need more information.

### 2. Test Thoroughly

Always test notifications before relying on them for critical alerts.

### 3. Use Color Coding

Consistent colors help quickly identify severity:

- 🔴 Red: Errors, outages
- 🟡 Yellow: Warnings
- 🟢 Green: Success, recovery
- 🔵 Blue: Informational

### 4. Group Related Events

Configure multiple providers for different event types:

- Critical alerts → Discord (with mentions)
- Info notifications → Slack (general channel)
- All events → Gotify (personal alerts)

### 5. Rate Limit Awareness

Be mindful of service limits:

- **Discord**: 5 requests per 2 seconds per webhook
- **Slack**: 1 request per second per workspace
- **Gotify**: No strict limits (self-hosted)

### 6. Keep Templates Maintainable

- Document custom templates
- Version control your templates
- Test after service updates

## Advanced Use Cases

### Multi-Channel Routing

Create separate providers for different severity levels:

```
Provider: Discord Critical
Events: uptime_down, ssl_failure
Template: Custom with @everyone mention

Provider: Slack Info
Events: ssl_renewal, backup_success
Template: Minimal

Provider: Gotify All
Events: * (all)
Template: Detailed
```

### Conditional Formatting

Use template logic (if supported by your service):

```json
{
  "embeds": [{
    "title": "{{.Title}}",
    "description": "{{.Message}}",
    "color": {{if eq .Severity "error"}}15158332{{else}}2326507{{end}}
  }]
}
```

### Integration with Automation

Forward notifications to automation tools:

```json
{
  "webhook_type": "charon_notification",
  "trigger_workflow": true,
  "data": {
    "event": "{{.EventType}}",
    "host": "{{.HostName}}",
    "action_required": {{if eq .Severity "error"}}true{{else}}false{{end}}
  }
}
```

## Additional Resources

- [Discord Webhook Documentation](https://discord.com/developers/docs/resources/webhook)
- [Slack Block Kit Builder](https://api.slack.com/block-kit)
- [Gotify API Documentation](https://gotify.net/docs/)
- [Charon Security Guide](../security.md)

## Need Help?

- 💬 [Ask in Discussions](https://github.com/Wikid82/charon/discussions)
- 🐛 [Report Issues](https://github.com/Wikid82/charon/issues)
- 📖 [View Full Documentation](https://wikid82.github.io/charon/)
