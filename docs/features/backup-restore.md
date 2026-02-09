---
title: Backup & Restore
description: Easy configuration backup and restoration
---

# Backup & Restore

Your configuration is valuable. Charon makes it easy to backup your entire setup and restore it when needed—whether you're migrating to new hardware or recovering from a problem.

## Overview

Charon provides automatic configuration backups and one-click restore functionality. Your proxy hosts, SSL certificates, access lists, and settings are all preserved, ensuring you can recover quickly from any situation.

Backups are stored within the Charon data directory and can be downloaded for off-site storage.

## Why Use This

- **Disaster Recovery**: Restore your entire configuration in seconds
- **Migration Made Easy**: Move to new hardware without reconfiguring
- **Change Confidence**: Make changes knowing you can roll back
- **Audit Trail**: Keep historical snapshots of your configuration

## What Gets Backed Up

| Component | Included |
|-----------|----------|
| **Database** | All proxy hosts, redirects, streams, and 404 hosts |
| **SSL Certificates** | Let's Encrypt certificates and custom certificates |
| **Access Lists** | All access control configurations |
| **Users** | User accounts and permissions |
| **Settings** | Application preferences and configurations |
| **CrowdSec Config** | Security settings and custom rules |

## Creating Backups

### Automatic Backups

Charon creates automatic backups:

- Before major configuration changes
- On a configurable schedule (default: daily)
- Before version upgrades

### Manual Backups

To create a manual backup:

1. Navigate to **Settings** → **Backup**
2. Click **Create Backup**
3. Optionally download the backup file for off-site storage

## Restoring from Backup

To restore a previous configuration:

1. Navigate to **Settings** → **Backup**
2. Select the backup to restore from the list
3. Click **Restore**
4. Confirm the restoration

> **Note**: Restoring a backup will overwrite current settings. Consider creating a backup of your current state first.

## Backup Retention

Charon manages backup storage automatically:

- **Automatic backups**: Retained for 30 days
- **Manual backups**: Retained indefinitely until deleted
- **Pre-upgrade backups**: Retained for 90 days

Configure retention settings in **Settings** → **Backup** → **Retention Policy**.

## Best Practices

1. **Download backups regularly** for off-site storage
2. **Test restores** periodically to ensure backups are valid
3. **Backup before changes** when modifying critical configurations
4. **Label manual backups** with descriptive names

## Related

- [Zero-Downtime Updates](live-reload.md)
- [Settings](../getting-started/configuration.md)
- [Back to Features](../features.md)
