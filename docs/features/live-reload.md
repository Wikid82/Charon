---
title: Zero-Downtime Updates
description: Make changes without interrupting your users
---

# Zero-Downtime Updates

Make changes without interrupting your users. Update domains, modify security rules, or add new services instantly. Your sites stay up while you work—no container restarts needed.

## Overview

Charon leverages Caddy's live reload capability to apply configuration changes without dropping connections. When you save changes in the UI, Caddy gracefully transitions to the new configuration while maintaining all active connections.

This means your users experience zero interruption—even during significant configuration changes.

## Why Use This

- **No Downtime**: Active connections remain unaffected
- **Instant Changes**: New configuration takes effect immediately
- **Safe Iteration**: Make frequent adjustments without risk
- **Production Friendly**: Update live systems confidently

## How It Works

When you save configuration changes:

1. Charon generates updated Caddy configuration
2. Caddy validates the new configuration
3. If valid, Caddy atomically swaps to the new config
4. Existing connections continue on old config until complete
5. New connections use the updated configuration

The entire process typically completes in milliseconds.

## What Can Be Changed Live

These changes apply instantly without any restart:

| Change Type | Live Reload |
|-------------|-------------|
| Add/remove proxy hosts | ✅ Yes |
| Modify upstream servers | ✅ Yes |
| Update SSL certificates | ✅ Yes |
| Change access lists | ✅ Yes |
| Modify headers | ✅ Yes |
| Update redirects | ✅ Yes |
| Add/remove domains | ✅ Yes |

## CrowdSec Integration Note

> **Important**: CrowdSec integration requires a one-time container restart when first enabled or when changing the CrowdSec API endpoint.

After the initial setup, CrowdSec decisions update automatically without restart. Only the connection to the CrowdSec API requires the restart.

To minimize disruption:

1. Configure CrowdSec during a maintenance window
2. After restart, all future updates are live

## Validation and Rollback

Charon validates all configuration changes before applying:

- **Syntax Validation**: Catches configuration errors
- **Connection Testing**: Verifies upstream availability
- **Automatic Rollback**: Invalid configs are rejected

If validation fails, your current configuration remains active and an error message explains the issue.

## Monitoring Changes

View configuration change history:

1. Check the **Real-Time Logs** for reload events
2. Review **Settings** → **Backup** for configuration snapshots

## Related

- [Backup & Restore](backup-restore.md)
- [Real-Time Logs](logs.md)
- [CrowdSec Integration](crowdsec.md)
- [Back to Features](../features.md)
