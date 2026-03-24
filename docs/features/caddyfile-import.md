---
title: Caddyfile Import
description: Import existing Caddyfile configurations with one click
category: migration
---

# Caddyfile Import

Migrating from another Caddy setup? Import your existing Caddyfile configurations with one click. Your existing work transfers seamlessly—no need to start from scratch.

## Overview

Caddyfile import parses your existing Caddy configuration files and converts them into Charon-managed hosts. This enables smooth migration from standalone Caddy installations, other Caddy-based tools, or configuration backups.

### Supported Configurations

- **Reverse Proxy Sites**: Domain → backend mappings
- **File Server Sites**: Static file hosting configurations
- **TLS Settings**: Certificate paths and ACME settings
- **Headers**: Custom header configurations
- **Redirects**: Redirect rules and rewrites

## Why Use This

### Preserve Existing Work

- Don't rebuild configurations from scratch
- Maintain proven routing rules
- Keep customizations intact

### Reduce Migration Risk

- Preview imports before applying
- Identify conflicts and duplicates
- Rollback if issues occur

### Accelerate Adoption

- Evaluate Charon without commitment
- Run imports on staging first
- Gradual migration at your pace

## How to Import

### Step 1: Access Import Tool

1. Navigate to **Settings** → **Import / Export**
2. Click **Import Caddyfile**

### Step 2: Provide Configuration

Choose one of three methods:

**Paste Content:**

```
example.com {
    reverse_proxy localhost:3000
}

api.example.com {
    reverse_proxy localhost:8080
}
```

**Upload File:**

- Click **Choose File**
- Select your Caddyfile

**Fetch from URL:**

- Enter URL to raw Caddyfile content
- Useful for version-controlled configurations

### Step 3: Preview and Confirm

The import preview shows:

- **Hosts Found**: Number of site blocks detected
- **Parse Warnings**: Non-fatal issues or unsupported directives
- **Conflicts**: Domains that already exist in Charon

### Step 4: Execute Import

Click **Import** to create hosts. The process handles each host individually—one failure doesn't block others.

## Import Results Modal

After import completes, a summary modal displays:

| Category | Description |
|----------|-------------|
| **Created** | New hosts added to Charon |
| **Updated** | Existing hosts modified (if overwrite enabled) |
| **Skipped** | Hosts skipped due to conflicts or errors |
| **Warnings** | Non-blocking issues to review |

### Example Results

```
Import Complete

✓ Created: 12 hosts
↻ Updated: 3 hosts
○ Skipped: 2 hosts
⚠ Warnings: 1

Details:
  ✓ example.com → localhost:3000
  ✓ api.example.com → localhost:8080
  ○ old.example.com (already exists, overwrite disabled)
  ⚠ staging.example.com (unsupported directive: php_fastcgi)
```

## Configuration Options

### Overwrite Existing

| Setting | Behavior |
|---------|----------|
| **Off** (default) | Skip hosts that already exist |
| **On** | Replace existing hosts with imported configuration |

### Import Disabled Hosts

Create hosts but leave them disabled for review before enabling.

### TLS Handling

| Source TLS Setting | Charon Behavior |
|--------------------|-----------------|
| ACME configured | Enable Let's Encrypt |
| Custom certificates | Create host, flag for manual cert upload |
| No TLS | Create HTTP-only host |

## Migration from Other Caddy Setups

### From Caddy Standalone

1. Locate your Caddyfile (typically `/etc/caddy/Caddyfile`)
2. Copy contents or upload file
3. Import into Charon
4. Verify hosts work correctly
5. Point DNS to Charon
6. Decommission old Caddy

### From Other Management Tools

Export Caddyfile from your current tool, then import into Charon. Most Caddy-based tools provide export functionality.

### Partial Migrations

Import specific site blocks by editing the Caddyfile before import. Remove sites you want to migrate later or manage separately.

## Limitations

Some Caddyfile features require manual configuration after import:

- Custom plugins/modules
- Complex matcher expressions
- Snippet references (imported inline)
- Global options (applied separately)

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Parse error | Check Caddyfile syntax validity |
| Missing hosts | Ensure site blocks have valid domains |
| TLS warnings | Configure certificates manually post-import |
| Duplicate domains | Enable overwrite or rename in source |

## Related

- [Web UI](web-ui.md) - Managing imported hosts
- [SSL Certificates](ssl-certificates.md) - Certificate configuration
- [Back to Features](../features.md)
