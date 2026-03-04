# Custom DNS Provider Plugins

Charon supports extending its DNS provider capabilities through a plugin system. This guide covers installation and usage of custom DNS provider plugins.

## Platform Limitations

**Important:** Go plugins are only supported on **Linux** and **macOS**. Windows users must rely on built-in DNS providers.

- **Supported:** Linux (x86_64, ARM64), macOS (x86_64, ARM64)
- **Not Supported:** Windows (any architecture)

## Security Considerations

### Critical Security Warnings

**⚠️ Plugins Execute In-Process**

Custom plugins run directly within the Charon process with full access to:

- All system resources and memory
- Database credentials
- API tokens and secrets
- File system access with Charon's permissions

**Only install plugins from trusted sources.**

### Security Best Practices

1. **Verify Plugin Source:** Only download plugins from official repositories or trusted developers
2. **Check Signatures:** Use signature verification (see Configuration section)
3. **Review Code:** If possible, review plugin source code before building
4. **Secure Permissions:** Plugin directory must not be world-writable (enforced automatically)
5. **Isolate Environment:** Consider running Charon in a container with restricted permissions
6. **Regular Updates:** Keep plugins updated to receive security patches

### Signature Verification

Configure signature verification in your Charon configuration:

```yaml
plugins:
  directory: /path/to/plugins
  allowed_signatures:
    powerdns: "sha256:abc123def456..."
    custom-provider: "sha256:789xyz..."
```

To generate a signature for a plugin:

```bash
sha256sum powerdns.so
# Output: abc123def456... powerdns.so
```

## Installation

### Prerequisites

- Charon must be built with CGO enabled (`CGO_ENABLED=1`)
- Go version must match between Charon and plugins (critical for compatibility)
- Plugin directory must exist with secure permissions

### Installation Steps

1. **Obtain the Plugin File**

   Download the `.so` file for your platform:

   ```bash
   curl https://example.com/plugins/powerdns-linux-amd64.so -O powerdns.so
   ```

2. **Verify Plugin Integrity (Recommended)**

   Check the SHA-256 signature:

   ```bash
   sha256sum powerdns.so
   # Compare with published signature
   ```

3. **Copy to Plugin Directory**

   ```bash
   sudo mkdir -p /etc/charon/plugins
   sudo cp powerdns.so /etc/charon/plugins/
   sudo chmod 755 /etc/charon/plugins/powerdns.so
   sudo chown root:root /etc/charon/plugins/powerdns.so
   ```

4. **Configure Charon**

   Edit your Charon configuration file:

   ```yaml
   plugins:
     directory: /etc/charon/plugins
     # Optional: Enable signature verification
     allowed_signatures:
       powerdns: "sha256:your-signature-here"
   ```

5. **Restart Charon**

   ```bash
   sudo systemctl restart charon
   ```

6. **Verify Plugin Loading**

   Check Charon logs:

   ```bash
   sudo journalctl -u charon -f | grep -i plugin
   ```

   Expected output:

   ```
   INFO Loaded DNS provider plugin type=powerdns name="PowerDNS" version="1.0.0"
   INFO Loaded 1 external DNS provider plugins (0 failed)
   ```

### Docker Installation

When running Charon in Docker:

1. **Mount Plugin Directory**

   ```yaml
   # docker-compose.yml
   services:
     charon:
       image: charon:latest
       volumes:
         - ./plugins:/etc/charon/plugins:ro
       environment:
         - PLUGIN_DIR=/etc/charon/plugins
   ```

2. **Build with Plugins**

   Alternatively, include plugins in your Docker image:

   ```dockerfile
   FROM charon:latest
   COPY plugins/*.so /etc/charon/plugins/
   ```

## Using Custom Providers

Once a plugin is installed and loaded, it appears in the DNS provider list alongside built-in providers.

### Discovering Loaded Plugins via API

Query available provider types to see all registered providers (built-in and plugins):

```bash
curl https://charon.example.com/api/v1/dns-providers/types \
  -H "Authorization: Bearer YOUR-TOKEN"
```

**Response:**

```json
{
  "types": [
    {
      "type": "cloudflare",
      "name": "Cloudflare",
      "description": "Cloudflare DNS provider",
      "documentation_url": "https://developers.cloudflare.com/api/",
      "is_built_in": true,
      "fields": [...]
    },
    {
      "type": "powerdns",
      "name": "PowerDNS",
      "description": "PowerDNS Authoritative Server with HTTP API",
      "documentation_url": "https://doc.powerdns.com/authoritative/http-api/",
      "is_built_in": false,
      "fields": [...]
    }
  ]
}
```

**Key fields:**

| Field | Description |
|-------|-------------|
| `is_built_in` | `true` = compiled into Charon, `false` = external plugin |
| `fields` | Credential field specifications for the UI form |

### Via Web UI

1. Navigate to **Settings** → **DNS Providers**
2. Click **Add Provider**
3. Select your custom provider from the dropdown
4. Enter required credentials
5. Click **Test Connection** to verify
6. Save the provider

### Via API

```bash
curl -X POST https://charon.example.com/api/admin/dns-providers \
  -H "Authorization: Bearer YOUR-TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "powerdns",
    "credentials": {
      "api_url": "https://pdns.example.com:8081",
      "api_key": "your-api-key",
      "server_id": "localhost"
    }
  }'
```

## Example: PowerDNS Plugin

The PowerDNS plugin demonstrates a complete DNS provider implementation.

### Required Credentials

- **API URL:** PowerDNS HTTP API endpoint (e.g., `https://pdns.example.com:8081`)
- **API Key:** X-API-Key header value for authentication

### Optional Credentials

- **Server ID:** PowerDNS server identifier (default: `localhost`)

### Configuration Example

```json
{
  "type": "powerdns",
  "credentials": {
    "api_url": "https://pdns.example.com:8081",
    "api_key": "your-secret-key",
    "server_id": "ns1"
  }
}
```

### Caddy Integration

The plugin automatically configures Caddy's DNS challenge for Let's Encrypt:

```json
{
  "name": "powerdns",
  "api_url": "https://pdns.example.com:8081",
  "api_key": "your-secret-key",
  "server_id": "ns1"
}
```

### Timeouts

- **Propagation Timeout:** 60 seconds
- **Polling Interval:** 2 seconds

## Plugin Management

### Listing Loaded Plugins

**Via Types Endpoint (Recommended):**

Filter for plugins using `is_built_in: false`:

```bash
curl https://charon.example.com/api/v1/dns-providers/types \
  -H "Authorization: Bearer YOUR-TOKEN" | jq '.types[] | select(.is_built_in == false)'
```

**Via Plugins Endpoint:**

Get detailed plugin metadata including version and author:

```bash
curl https://charon.example.com/api/admin/plugins \
  -H "Authorization: Bearer YOUR-TOKEN"
```

Response:

```json
{
  "plugins": [
    {
      "type": "powerdns",
      "name": "PowerDNS",
      "description": "PowerDNS Authoritative Server with HTTP API",
      "version": "1.0.0",
      "author": "Charon Community",
      "is_built_in": false,
      "go_version": "go1.23.4",
      "interface_version": "v1"
    }
  ]
}
```

### Reloading Plugins

To reload plugins without restarting Charon:

```bash
curl -X POST https://charon.example.com/api/admin/plugins/reload \
  -H "Authorization: Bearer YOUR-TOKEN"
```

**Note:** Due to Go runtime limitations, plugin code remains in memory even after unloading. A full restart is required to completely unload plugin code.

### Unloading a Plugin

```bash
curl -X DELETE https://charon.example.com/api/admin/plugins/powerdns \
  -H "Authorization: Bearer YOUR-TOKEN"
```

## Troubleshooting

### Plugin Not Loading

**Check Go Version Compatibility:**

```bash
go version
# Must match the version shown in plugin metadata
```

**Check Plugin File Permissions:**

```bash
ls -la /etc/charon/plugins/
# Should be 755 or 644, not world-writable
```

**Check Charon Logs:**

```bash
sudo journalctl -u charon -n 100 | grep -i plugin
```

### Common Errors

#### `plugin was built with a different version of Go`

**Cause:** Plugin compiled with different Go version than Charon

**Solution:** Rebuild plugin with matching Go version or rebuild Charon

#### `plugin not in allowlist`

**Cause:** Signature verification enabled, but plugin not in allowed list

**Solution:** Add plugin signature to `allowed_signatures` configuration

#### `signature mismatch`

**Cause:** Plugin file signature doesn't match expected value

**Solution:** Verify plugin file integrity, re-download if corrupted

#### `missing 'Plugin' symbol`

**Cause:** Plugin doesn't export required `Plugin` variable

**Solution:** Rebuild plugin with correct exported symbol (see developer guide)

#### `interface version mismatch`

**Cause:** Plugin built against incompatible interface version

**Solution:** Update plugin to match Charon's interface version

### Directory Permission Errors

If Charon reports "directory has insecure permissions":

```bash
# Fix directory permissions
sudo chmod 755 /etc/charon/plugins

# Ensure not world-writable
sudo chmod -R o-w /etc/charon/plugins
```

## Performance Considerations

- **Startup Time:** Plugin loading adds 10-50ms per plugin to startup time
- **Memory:** Each plugin uses 1-5MB of additional memory
- **Runtime:** Plugin calls have minimal overhead (nanoseconds)

## Compatibility Matrix

| Charon Version | Interface Version | Go Version Required |
|----------------|-------------------|---------------------|
| 1.0.x          | v1                | 1.23.x              |
| 1.1.x          | v1                | 1.23.x              |
| 2.0.x          | v2                | 1.24.x              |

**Always use plugins built for your Charon interface version.**

## Support

### Getting Help

- **GitHub Discussions:** <https://github.com/Wikid82/charon/discussions>
- **Issue Tracker:** <https://github.com/Wikid82/charon/issues>
- **Documentation:** <https://docs.charon.example.com>

### Reporting Issues

When reporting plugin issues, include:

1. Charon version and Go version
2. Plugin name and version
3. Operating system and architecture
4. Complete error logs
5. Plugin metadata (from API response)

## See Also

- [Plugin Security Guide](./plugin-security.md)
- [Plugin Development Guide](../development/plugin-development.md)
- [DNS Provider Configuration](./dns-providers.md)
- [Security Best Practices](../../SECURITY.md)
