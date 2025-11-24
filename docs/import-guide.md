# Caddyfile Import Guide

This guide explains how to import existing Caddyfiles into CaddyProxyManager+, handle conflicts, and troubleshoot common issues.

## Table of Contents

- [Overview](#overview)
- [Import Methods](#import-methods)
- [Import Workflow](#import-workflow)
- [Conflict Resolution](#conflict-resolution)
- [Supported Caddyfile Syntax](#supported-caddyfile-syntax)
- [Limitations](#limitations)
- [Troubleshooting](#troubleshooting)
- [Examples](#examples)

## Overview

CaddyProxyManager+ can import existing Caddyfiles and convert them into managed proxy host configurations. This is useful when:

- Migrating from standalone Caddy to CaddyProxyManager+
- Importing configurations from other systems
- Bulk importing multiple proxy hosts
- Sharing configurations between environments

## Import Methods

### Method 1: File Upload

1. Navigate to **Import Caddyfile** page
2. Click **Choose File** button
3. Select your Caddyfile (any text file)
4. Click **Upload**

### Method 2: Paste Content

1. Navigate to **Import Caddyfile** page
2. Click **Paste Caddyfile** tab
3. Paste your Caddyfile content into the textarea
4. Click **Preview Import**

## Import Workflow

The import process follows these steps:

### 1. Upload/Paste

Upload your Caddyfile or paste the content directly.

```caddyfile
# Example Caddyfile
example.com {
    reverse_proxy localhost:8080
}

api.example.com {
    reverse_proxy https://backend:9000
}
```

### 2. Parsing

The system parses your Caddyfile and extracts:
- Domain names
- Reverse proxy directives
- TLS settings
- Headers and other directives

**Parsing States:**
- ✅ **Success** - All hosts parsed correctly
- ⚠️ **Partial** - Some hosts parsed, others failed
- ❌ **Failed** - Critical parsing error

### 3. Preview

Review the parsed configurations:

| Domain | Forward Host | Forward Port | SSL | Status |
|--------|--------------|--------------|-----|--------|
| example.com | localhost | 8080 | No | New |
| api.example.com | backend | 9000 | Yes | New |

### 4. Conflict Detection

The system checks if any imported domains already exist:

- **No Conflicts** - All domains are new, safe to import
- **Conflicts Found** - One or more domains already exist

### 5. Conflict Resolution

For each conflict, choose an action:

| Domain | Existing Config | New Config | Action |
|--------|-----------------|------------|--------|
| example.com | localhost:3000 | localhost:8080 | [Keep Existing ▼] |

**Resolution Options:**
- **Keep Existing** - Don't import this host, keep current configuration
- **Overwrite** - Replace existing configuration with imported one
- **Skip** - Don't import this host, keep existing unchanged
- **Create New** - Import as a new host with modified domain name

### 6. Commit

Once all conflicts are resolved, click **Commit Import** to finalize.

**Post-Import:**
- Imported hosts appear in Proxy Hosts list
- Configurations are saved to database
- Caddy configs are generated automatically

## Conflict Resolution

### Strategy: Keep Existing

Use when you want to preserve your current configuration and ignore the imported one.

```
Current:  example.com → localhost:3000
Imported: example.com → localhost:8080
Result:   example.com → localhost:3000 (unchanged)
```

### Strategy: Overwrite

Use when the imported configuration is newer or more correct.

```
Current:  example.com → localhost:3000
Imported: example.com → localhost:8080
Result:   example.com → localhost:8080 (replaced)
```

### Strategy: Skip

Same as "Keep Existing" - imports everything except conflicting hosts.

### Strategy: Create New (Future)

Renames the imported host to avoid conflicts (e.g., `example.com` → `example-2.com`).

## Supported Caddyfile Syntax

### Basic Reverse Proxy

```caddyfile
example.com {
    reverse_proxy localhost:8080
}
```

**Parsed as:**
- Domain: `example.com`
- Forward Host: `localhost`
- Forward Port: `8080`
- Forward Scheme: `http`

### HTTPS Upstream

```caddyfile
secure.example.com {
    reverse_proxy https://backend:9000
}
```

**Parsed as:**
- Domain: `secure.example.com`
- Forward Host: `backend`
- Forward Port: `9000`
- Forward Scheme: `https`

### Multiple Domains

```caddyfile
example.com, www.example.com {
    reverse_proxy localhost:8080
}
```

**Parsed as:**
- Domain: `example.com, www.example.com`
- Forward Host: `localhost`
- Forward Port: `8080`

### TLS Configuration

```caddyfile
example.com {
    tls internal
    reverse_proxy localhost:8080
}
```

**Parsed as:**
- SSL Forced: `true`
- TLS provider: `internal` (self-signed)

### Headers and Directives

```caddyfile
example.com {
    header {
        X-Custom-Header "value"
    }
    reverse_proxy localhost:8080 {
        header_up Host {host}
    }
}
```

**Note:** Custom headers and advanced directives are stored in the raw CaddyConfig but may not be editable in the UI initially.

## Limitations

### Current Limitations

1. **Path-based routing** - Not yet supported
   ```caddyfile
   example.com {
       route /api/* {
           reverse_proxy localhost:8080
       }
       route /static/* {
           file_server
       }
   }
   ```

2. **File server blocks** - Only reverse_proxy supported
   ```caddyfile
   static.example.com {
       file_server
       root * /var/www/html
   }
   ```

3. **Advanced matchers** - Basic domain matching only
   ```caddyfile
   @api {
       path /api/*
       header X-API-Key *
   }
   reverse_proxy @api localhost:8080
   ```

4. **Import statements** - Must be resolved before import
   ```caddyfile
   import snippets/common.caddy
   ```

5. **Environment variables** - Must be hardcoded
   ```caddyfile
   {$DOMAIN} {
       reverse_proxy {$BACKEND_HOST}
   }
   ```

### Workarounds

- **Path routing**: Create multiple proxy hosts per path
- **File server**: Use separate Caddy instance or static host tool
- **Matchers**: Manually configure in Caddy after import
- **Imports**: Flatten your Caddyfile before importing
- **Variables**: Replace with actual values before import

## Troubleshooting

### Error: "Failed to parse Caddyfile"

**Cause:** Invalid Caddyfile syntax

**Solution:**
1. Validate your Caddyfile with `caddy validate --config Caddyfile`
2. Check for missing braces `{}`
3. Ensure reverse_proxy directives are properly formatted

### Error: "No hosts found in Caddyfile"

**Cause:** Only contains directives without reverse_proxy blocks

**Solution:**
- Ensure you have at least one `reverse_proxy` directive
- Remove file_server-only blocks
- Add domain blocks with reverse_proxy

### Warning: "Some hosts could not be imported"

**Cause:** Partial import with unsupported features

**Solution:**
- Review the preview to see which hosts failed
- Simplify complex directives
- Import compatible hosts, add others manually

### Conflict Resolution Stuck

**Cause:** Not all conflicts have resolution selected

**Solution:**
- Ensure every conflicting host has a resolution dropdown selection
- The "Commit Import" button enables only when all conflicts are resolved

## Examples

### Example 1: Simple Migration

**Original Caddyfile:**
```caddyfile
app.example.com {
    reverse_proxy localhost:3000
}

api.example.com {
    reverse_proxy localhost:8080
}
```

**Import Result:**
- 2 hosts imported successfully
- No conflicts
- Ready to use immediately

### Example 2: HTTPS Upstream

**Original Caddyfile:**
```caddyfile
secure.example.com {
    reverse_proxy https://internal.corp:9000 {
        transport http {
            tls_insecure_skip_verify
        }
    }
}
```

**Import Result:**
- Domain: `secure.example.com`
- Forward: `https://internal.corp:9000`
- Note: `tls_insecure_skip_verify` stored in raw config

### Example 3: Multi-domain with Conflict

**Original Caddyfile:**
```caddyfile
example.com, www.example.com {
    reverse_proxy localhost:8080
}
```

**Existing Configuration:**
- `example.com` already points to `localhost:3000`

**Resolution:**
1. System detects conflict on `example.com`
2. Choose **Overwrite** to use new config
3. Commit import
4. Result: `example.com, www.example.com → localhost:8080`

### Example 4: Complex Setup (Partial Import)

**Original Caddyfile:**
```caddyfile
# Supported
app.example.com {
    reverse_proxy localhost:3000
}

# Supported
api.example.com {
    reverse_proxy https://backend:8080
}

# NOT supported (file server)
static.example.com {
    file_server
    root * /var/www
}

# NOT supported (path routing)
multi.example.com {
    route /api/* {
        reverse_proxy localhost:8080
    }
    route /web/* {
        reverse_proxy localhost:3000
    }
}
```

**Import Result:**
- ✅ `app.example.com` imported
- ✅ `api.example.com` imported
- ❌ `static.example.com` skipped (file_server not supported)
- ❌ `multi.example.com` skipped (path routing not supported)
- **Action:** Add unsupported hosts manually through UI or keep separate Caddyfile

## Best Practices

1. **Validate First** - Run `caddy validate` before importing
2. **Backup** - Keep a backup of your original Caddyfile
3. **Simplify** - Remove unsupported directives before import
4. **Test Small** - Import a few hosts first to verify
5. **Review Preview** - Always check the preview before committing
6. **Resolve Conflicts Carefully** - Understand impact before overwriting
7. **Document Custom Config** - Note any advanced directives that can't be edited in UI

## Getting Help

If you encounter issues:

1. Check this guide's [Troubleshooting](#troubleshooting) section
2. Review [Supported Syntax](#supported-caddyfile-syntax)
3. Open an issue on GitHub with:
   - Your Caddyfile (sanitized)
   - Error messages
   - Expected vs actual behavior

## Future Enhancements

Planned improvements to import functionality:

- [ ] Path-based routing support
- [ ] Custom header import/export
- [ ] Environment variable resolution
- [ ] Import from URL
- [ ] Export to Caddyfile
- [ ] Diff view for conflicts
- [ ] Batch import from multiple files
- [ ] Import validation before upload
