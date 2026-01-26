# Plugin Security Guide

This guide covers security configuration and deployment patterns for Charon's plugin system. For general plugin installation and usage, see [Custom Plugins](./custom-plugins.md).

## Overview

Charon supports external DNS provider plugins via Go's plugin system. Because plugins execute **in-process** with full memory access, they must be treated as trusted code. This guide explains how to:

- Configure signature-based allowlisting
- Deploy plugins securely in containers
- Mitigate common attack vectors

---

## Plugin Signature Allowlisting

Charon supports SHA-256 signature verification to ensure only approved plugins are loaded.

### Environment Variable

```bash
CHARON_PLUGIN_SIGNATURES='{"pluginname": "sha256:..."}'
```

**Key format**: Plugin name **without** the `.so` extension.

### Behavior Matrix

| `CHARON_PLUGIN_SIGNATURES` Value | Behavior |
|----------------------------------|----------|
| Unset or empty (`""`) | **Permissive mode** — All plugins are loaded (backward compatible) |
| Set to `{}` | **Strict block-all** — No external plugins are loaded |
| Set with entries | **Allowlist mode** — Only listed plugins with matching signatures are loaded |

### Examples

**Permissive mode (default)**:
```bash
# Unset — all plugins load without verification
unset CHARON_PLUGIN_SIGNATURES
```

**Strict block-all**:
```bash
# Empty object — no external plugins will load
export CHARON_PLUGIN_SIGNATURES='{}'
```

**Allowlist specific plugins**:
```bash
# Only powerdns and custom-provider plugins are allowed
export CHARON_PLUGIN_SIGNATURES='{"powerdns": "sha256:a1b2c3d4...", "custom-provider": "sha256:e5f6g7h8..."}'
```

---

## Generating Plugin Signatures

To add a plugin to your allowlist, compute its SHA-256 signature:

```bash
sha256sum myplugin.so | awk '{print "sha256:" $1}'
```

**Example output**:
```
sha256:a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0e1f2
```

Use this value in your `CHARON_PLUGIN_SIGNATURES` JSON:

```bash
export CHARON_PLUGIN_SIGNATURES='{"myplugin": "sha256:a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0e1f2"}'
```

> **⚠️ Important**: The key is the plugin name **without** `.so`. Use `myplugin`, not `myplugin.so`.

---

## Container Deployment Recommendations

### Read-Only Plugin Mount (Critical)

**Always mount the plugin directory as read-only in production**:

```yaml
# docker-compose.yml
services:
  charon:
    image: charon:latest
    volumes:
      - ./plugins:/app/plugins:ro  # Read-only mount
    environment:
      - CHARON_PLUGINS_DIR=/app/plugins
      - CHARON_PLUGIN_SIGNATURES={"powerdns": "sha256:..."}
```

This prevents runtime modification of plugin files, mitigating:
- Time-of-check to time-of-use (TOCTOU) attacks
- Malicious plugin replacement after signature verification

### Non-Root Execution

Run Charon as a non-root user:

```yaml
# docker-compose.yml
services:
  charon:
    image: charon:latest
    user: "1000:1000"  # Non-root user
    # ...
```

Or in Dockerfile:
```dockerfile
FROM charon:latest
USER charon
```

### Directory Permissions

Plugin directories must **not** be world-writable. Charon enforces this at startup.

| Permission | Result |
|------------|--------|
| `0755` or stricter | ✅ Allowed |
| `0777` (world-writable) | ❌ Rejected — plugin loading disabled |

**Set secure permissions**:
```bash
chmod 755 /path/to/plugins
chmod 644 /path/to/plugins/*.so  # Or 755 for executable
```

### Complete Secure Deployment Example

```yaml
# docker-compose.production.yml
services:
  charon:
    image: charon:latest
    user: "1000:1000"
    read_only: true
    security_opt:
      - no-new-privileges:true
    volumes:
      - ./plugins:/app/plugins:ro
      - ./data:/app/data
    environment:
      - CHARON_PLUGINS_DIR=/app/plugins
      - CHARON_PLUGIN_SIGNATURES={"powerdns": "sha256:abc123..."}
    tmpfs:
      - /tmp
```

---

## TOCTOU Mitigation

Time-of-check to time-of-use (TOCTOU) vulnerabilities occur when a file is modified between signature verification and loading. Mitigate with:

### 1. Read-Only Mounts (Primary Defense)

Mount the plugin directory as read-only (`:ro`). This prevents modification after startup.

### 2. Atomic File Replacement for Updates

When updating plugins, use atomic operations to avoid partial writes:

```bash
# 1. Copy new plugin to temporary location
cp new_plugin.so /tmp/plugin.so.new

# 2. Atomically replace the old plugin
mv /tmp/plugin.so.new /app/plugins/plugin.so

# 3. Restart Charon to reload plugins
docker compose restart charon
```

> **⚠️ Warning**: `cp` followed by direct write to the plugin directory is **not atomic** and creates a window for exploitation.

### 3. Signature Re-Verification on Reload

After updating plugins, always update your `CHARON_PLUGIN_SIGNATURES` with the new hash before restarting.

---

## Troubleshooting

### Checking if a Plugin Loaded

**Check startup logs**:
```bash
docker compose logs charon | grep -i plugin
```

**Expected success output**:
```
INFO Loaded DNS provider plugin type=powerdns name="PowerDNS" version="1.0.0"
INFO Loaded 1 external DNS provider plugins (0 failed)
```

**If using allowlist**:
```
INFO Plugin signature allowlist enabled with 2 entries
```

**Via API**:
```bash
curl http://localhost:8080/api/admin/plugins \
  -H "Authorization: Bearer YOUR-TOKEN"
```

### Common Error Messages

#### `plugin not in allowlist`

**Cause**: The plugin filename (without `.so`) is not in `CHARON_PLUGIN_SIGNATURES`.

**Solution**: Add the plugin to your allowlist:
```bash
# Get the signature
sha256sum powerdns.so | awk '{print "sha256:" $1}'

# Add to environment
export CHARON_PLUGIN_SIGNATURES='{"powerdns": "sha256:YOUR_HASH_HERE"}'
```

#### `signature mismatch for plugin`

**Cause**: The plugin file's SHA-256 hash doesn't match the allowlist.

**Solution**:
1. Verify you have the correct plugin file
2. Re-compute the signature: `sha256sum plugin.so`
3. Update `CHARON_PLUGIN_SIGNATURES` with the correct hash

#### `plugin directory has insecure permissions`

**Cause**: The plugin directory is world-writable (mode `0777` or similar).

**Solution**:
```bash
chmod 755 /path/to/plugins
chmod 644 /path/to/plugins/*.so
```

#### `invalid CHARON_PLUGIN_SIGNATURES JSON`

**Cause**: Malformed JSON in the environment variable.

**Solution**: Validate your JSON:
```bash
echo '{"powerdns": "sha256:abc123"}' | jq .
```

Common issues:
- Missing quotes around keys or values
- Trailing commas
- Single quotes instead of double quotes

#### Permission denied when loading plugin

**Cause**: File permissions too restrictive or ownership mismatch.

**Solution**:
```bash
# Check current permissions
ls -la /path/to/plugins/

# Fix permissions
chmod 644 /path/to/plugins/*.so
chown charon:charon /path/to/plugins/*.so
```

### Debugging Checklist

1. **Is the plugin directory configured?**
   ```bash
   echo $CHARON_PLUGINS_DIR
   ```

2. **Does the plugin file exist?**
   ```bash
   ls -la $CHARON_PLUGINS_DIR/*.so
   ```

3. **Are directory permissions secure?**
   ```bash
   stat -c "%a %n" $CHARON_PLUGINS_DIR
   # Should be 755 or stricter
   ```

4. **Is the signature correct?**
   ```bash
   sha256sum $CHARON_PLUGINS_DIR/myplugin.so
   ```

5. **Is the JSON valid?**
   ```bash
   echo "$CHARON_PLUGIN_SIGNATURES" | jq .
   ```

---

## Security Implications

### What Plugins Can Access

Plugins run **in-process** with Charon and have access to:

| Resource | Access Level |
|----------|--------------|
| System memory | Full read/write |
| Database credentials | Full access |
| API tokens and secrets | Full access |
| File system | Charon's permissions |
| Network | Unrestricted outbound |

### Risk Assessment

| Risk | Mitigation |
|------|------------|
| Malicious plugin code | Signature allowlisting, code review |
| Plugin replacement attack | Read-only mounts, atomic updates |
| World-writable directory | Automatic permission verification |
| Supply chain compromise | Verify plugin source, pin signatures |

### Best Practices Summary

1. ✅ **Enable signature allowlisting** in production
2. ✅ **Mount plugin directory read-only** (`:ro`)
3. ✅ **Run as non-root user**
4. ✅ **Use strict directory permissions** (`0755` or stricter)
5. ✅ **Verify plugin source** before deployment
6. ✅ **Update signatures** after plugin updates
7. ❌ **Never use permissive mode** in production
8. ❌ **Never install plugins from untrusted sources**

---

## See Also

- [Custom Plugins](./custom-plugins.md) — Plugin installation and usage
- [Security Policy](../../SECURITY.md) — Security reporting and policies
- [Plugin Development Guide](../development/plugin-development.md) — Building custom plugins
