# DNS Provider Plugin Development

This guide covers the technical details of developing custom DNS provider plugins for Charon.

## Overview

Charon uses Go's plugin system to dynamically load DNS provider implementations. Plugins implement the `ProviderPlugin` interface and are compiled as shared libraries (`.so` files).

### Architecture

```
┌─────────────────────────────────────────┐
│         Charon Core Process             │
│  ┌───────────────────────────────────┐  │
│  │   Global Provider Registry        │  │
│  ├───────────────────────────────────┤  │
│  │ Built-in Providers                │  │
│  │  - Cloudflare                     │  │
│  │  - DNSimple                       │  │
│  │  - Route53                        │  │
│  ├───────────────────────────────────┤  │
│  │ External Plugins (*.so)           │  │
│  │  - PowerDNS     [loaded]          │  │
│  │  - Custom       [loaded]          │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

## Platform Requirements

### Supported Platforms

- **Linux:** x86_64, ARM64 (primary target)
- **macOS:** x86_64, ARM64 (development/testing)
- **Windows:** Not supported (Go plugin limitation)

### Build Requirements

- **CGO:** Must be enabled (`CGO_ENABLED=1`)
- **Go Version:** Must match Charon's Go version exactly (currently 1.25.6+)
- **Compiler:** GCC/Clang for Linux, Xcode tools for macOS
- **Build Mode:** Must use `-buildmode=plugin`

## Interface Specification

### Interface Version

Current interface version: **v1**

The interface version is defined in `backend/pkg/dnsprovider/plugin.go`:

```go
const InterfaceVersion = "v1"
```

### Core Interface

All plugins must implement `dnsprovider.ProviderPlugin`:

```go
type ProviderPlugin interface {
    Type() string
    Metadata() ProviderMetadata
    Init() error
    Cleanup() error
    RequiredCredentialFields() []CredentialFieldSpec
    OptionalCredentialFields() []CredentialFieldSpec
    ValidateCredentials(creds map[string]string) error
    TestCredentials(creds map[string]string) error
    SupportsMultiCredential() bool
    BuildCaddyConfig(creds map[string]string) map[string]any
    BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any
    PropagationTimeout() time.Duration
    PollingInterval() time.Duration
}
```

### Method Reference

#### `Type() string`

Returns the unique provider identifier.

- Must be lowercase, alphanumeric with optional underscores
- Used as the key for registration and lookup
- Examples: `"powerdns"`, `"custom_dns"`, `"acme_dns"`

#### `Metadata() ProviderMetadata`

Returns descriptive information for UI display:

```go
type ProviderMetadata struct {
    Type             string `json:"type"`               // Same as Type()
    Name             string `json:"name"`               // Display name
    Description      string `json:"description"`        // Brief description
    DocumentationURL string `json:"documentation_url"`  // Help link
    Author           string `json:"author"`             // Plugin author
    Version          string `json:"version"`            // Plugin version
    IsBuiltIn        bool   `json:"is_built_in"`        // Always false for plugins
    GoVersion        string `json:"go_version"`         // Build Go version
    InterfaceVersion string `json:"interface_version"`  // Plugin interface version
}
```

**Required fields:** `Type`, `Name`, `Description`, `IsBuiltIn` (false), `GoVersion`, `InterfaceVersion`

#### `Init() error`

Called after the plugin is loaded, before registration.

Use for:

- Loading configuration files
- Validating environment
- Establishing persistent connections
- Resource allocation

Return an error to prevent registration.

#### `Cleanup() error`

Called before the plugin is unregistered (graceful shutdown).

Use for:

- Closing connections
- Flushing caches
- Releasing resources

**Note:** Due to Go runtime limitations, plugin code remains in memory after `Cleanup()`.

#### `RequiredCredentialFields() []CredentialFieldSpec`

Returns credential fields that must be provided.

Example:

```go
return []dnsprovider.CredentialFieldSpec{
    {
        Name:        "api_token",
        Label:       "API Token",
        Type:        "password",
        Placeholder: "Enter your API token",
        Hint:        "Found in your account settings",
    },
}
```

#### `OptionalCredentialFields() []CredentialFieldSpec`

Returns credential fields that may be provided.

Example:

```go
return []dnsprovider.CredentialFieldSpec{
    {
        Name:        "timeout",
        Label:       "Timeout (seconds)",
        Type:        "text",
        Placeholder: "30",
        Hint:        "API request timeout",
    },
}
```

#### `ValidateCredentials(creds map[string]string) error`

Validates credential format and presence (no network calls).

Example:

```go
func (p *PowerDNSProvider) ValidateCredentials(creds map[string]string) error {
    if creds["api_url"] == "" {
        return fmt.Errorf("api_url is required")
    }
    if creds["api_key"] == "" {
        return fmt.Errorf("api_key is required")
    }
    return nil
}
```

#### `TestCredentials(creds map[string]string) error`

Verifies credentials work with the provider API (may make network calls).

Example:

```go
func (p *PowerDNSProvider) TestCredentials(creds map[string]string) error {
    if err := p.ValidateCredentials(creds); err != nil {
        return err
    }

    // Test API connectivity
    url := creds["api_url"] + "/api/v1/servers"
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("X-API-Key", creds["api_key"])

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("API connection failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("API returned status %d", resp.StatusCode)
    }
    return nil
}
```

#### `SupportsMultiCredential() bool`

Indicates if the provider supports zone-specific credentials (Phase 3 feature).

Return `false` for most implementations:

```go
func (p *PowerDNSProvider) SupportsMultiCredential() bool {
    return false
}
```

#### `BuildCaddyConfig(creds map[string]string) map[string]any`

Constructs Caddy DNS challenge configuration.

The returned map is embedded into Caddy's TLS automation policy for ACME DNS-01 challenges.

Example:

```go
func (p *PowerDNSProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
    return map[string]any{
        "name":      "powerdns",
        "api_url":   creds["api_url"],
        "api_key":   creds["api_key"],
        "server_id": creds["server_id"],
    }
}
```

**Caddy Configuration Reference:** See [Caddy DNS Providers](https://github.com/caddy-dns)

#### `BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any`

Constructs zone-specific configuration (multi-credential mode).

Only called if `SupportsMultiCredential()` returns `true`.

Most plugins can simply delegate to `BuildCaddyConfig()`:

```go
func (p *PowerDNSProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
    return p.BuildCaddyConfig(creds)
}
```

#### `PropagationTimeout() time.Duration`

Returns the recommended DNS propagation wait time.

Typical values:

- **Fast providers:** 30-60 seconds (Cloudflare, PowerDNS)
- **Standard providers:** 60-120 seconds (DNSimple, Route53)
- **Slow providers:** 120-300 seconds (traditional DNS)

```go
func (p *PowerDNSProvider) PropagationTimeout() time.Duration {
    return 60 * time.Second
}
```

#### `PollingInterval() time.Duration`

Returns the recommended polling interval for DNS verification.

Typical values: 2-10 seconds

```go
func (p *PowerDNSProvider) PollingInterval() time.Duration {
    return 2 * time.Second
}
```

## Plugin Structure

### Minimal Plugin Template

```go
package main

import (
    "fmt"
    "runtime"
    "time"

    "github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// Plugin is the exported symbol that Charon looks for
var Plugin dnsprovider.ProviderPlugin = &MyProvider{}

type MyProvider struct{}

func (p *MyProvider) Type() string {
    return "myprovider"
}

func (p *MyProvider) Metadata() dnsprovider.ProviderMetadata {
    return dnsprovider.ProviderMetadata{
        Type:             "myprovider",
        Name:             "My DNS Provider",
        Description:      "Custom DNS provider implementation",
        DocumentationURL: "https://example.com/docs",
        Author:           "Your Name",
        Version:          "1.0.0",
        IsBuiltIn:        false,
        GoVersion:        runtime.Version(),
        InterfaceVersion: dnsprovider.InterfaceVersion,
    }
}

func (p *MyProvider) Init() error {
    return nil
}

func (p *MyProvider) Cleanup() error {
    return nil
}

func (p *MyProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
    return []dnsprovider.CredentialFieldSpec{
        {
            Name:        "api_key",
            Label:       "API Key",
            Type:        "password",
            Placeholder: "Enter your API key",
            Hint:        "Found in your account settings",
        },
    }
}

func (p *MyProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
    return []dnsprovider.CredentialFieldSpec{}
}

func (p *MyProvider) ValidateCredentials(creds map[string]string) error {
    if creds["api_key"] == "" {
        return fmt.Errorf("api_key is required")
    }
    return nil
}

func (p *MyProvider) TestCredentials(creds map[string]string) error {
    return p.ValidateCredentials(creds)
}

func (p *MyProvider) SupportsMultiCredential() bool {
    return false
}

func (p *MyProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
    return map[string]any{
        "name":    "myprovider",
        "api_key": creds["api_key"],
    }
}

func (p *MyProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
    return p.BuildCaddyConfig(creds)
}

func (p *MyProvider) PropagationTimeout() time.Duration {
    return 60 * time.Second
}

func (p *MyProvider) PollingInterval() time.Duration {
    return 5 * time.Second
}

func main() {}
```

### Project Layout

```
my-provider-plugin/
├── go.mod
├── go.sum
├── main.go
├── Makefile
└── README.md
```

### `go.mod` Requirements

```go
module github.com/yourname/charon-plugin-myprovider

go 1.25

require (
    github.com/Wikid82/charon v0.0.0-20240101000000-abcdef123456
)
```

**Important:** Use `replace` directive for local development:

```go
replace github.com/Wikid82/charon => /path/to/charon
```

## Building Plugins

### Build Command

```bash
CGO_ENABLED=1 go build -buildmode=plugin -o myprovider.so main.go
```

### Build Requirements

1. **CGO must be enabled:**

   ```bash
   export CGO_ENABLED=1
   ```

2. **Go version must match Charon:**

   ```bash
   go version
   # Must match Charon's build Go version
   ```

3. **Architecture must match:**

   ```bash
   # For cross-compilation
   GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildmode=plugin
   ```

### Makefile Example

```makefile
.PHONY: build clean install

PLUGIN_NAME = myprovider
OUTPUT = $(PLUGIN_NAME).so
INSTALL_DIR = /etc/charon/plugins

build:
 CGO_ENABLED=1 go build -buildmode=plugin -o $(OUTPUT) main.go

clean:
 rm -f $(OUTPUT)

install: build
 install -m 755 $(OUTPUT) $(INSTALL_DIR)/

test:
 go test -v ./...

lint:
 golangci-lint run

signature:
 @echo "SHA-256 Signature:"
 @sha256sum $(OUTPUT)
```

### Build Script

```bash
#!/bin/bash
set -e

PLUGIN_NAME="myprovider"
GO_VERSION=$(go version | awk '{print $3}')
CHARON_GO_VERSION="go1.25.6"

# Verify Go version
if [ "$GO_VERSION" != "$CHARON_GO_VERSION" ]; then
    echo "Warning: Go version mismatch"
    echo "  Plugin: $GO_VERSION"
    echo "  Charon: $CHARON_GO_VERSION"
    read -p "Continue? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Build plugin
echo "Building $PLUGIN_NAME.so..."
CGO_ENABLED=1 go build -buildmode=plugin -o "${PLUGIN_NAME}.so" main.go

# Generate signature
echo "Generating signature..."
sha256sum "${PLUGIN_NAME}.so" | tee "${PLUGIN_NAME}.so.sha256"

echo "Build complete!"
```

## Development Workflow

### 1. Set Up Development Environment

```bash
# Clone plugin template
git clone https://github.com/yourname/charon-plugin-template my-provider
cd my-provider

# Install dependencies
go mod download

# Set up local Charon dependency
echo 'replace github.com/Wikid82/charon => /path/to/charon' >> go.mod
go mod tidy
```

### 2. Implement Provider Interface

Edit `main.go` to implement all required methods.

### 3. Test Locally

```bash
# Build plugin
make build

# Copy to Charon plugin directory
cp myprovider.so /etc/charon/plugins/

# Restart Charon
systemctl restart charon

# Check logs
journalctl -u charon -f | grep plugin
```

### 4. Debug Plugin Loading

Enable debug logging in Charon:

```yaml
log:
  level: debug
```

Check for errors:

```bash
journalctl -u charon -n 100 | grep -i plugin
```

### 5. Test Credential Validation

```bash
curl -X POST http://localhost:8080/api/admin/dns-providers/test \
  -H "Content-Type: application/json" \
  -d '{
    "type": "myprovider",
    "credentials": {
      "api_key": "test-key"
    }
  }'
```

### 6. Test DNS Challenge

Configure a test domain to use your provider and request a certificate.

Monitor Caddy logs for DNS challenge execution:

```bash
docker logs charon-caddy -f | grep dns
```

## Best Practices

### Security

1. **Validate All Inputs:** Never trust credential data
2. **Use HTTPS:** Always use TLS for API connections
3. **Timeout Requests:** Set reasonable timeouts on all HTTP calls
4. **Sanitize Errors:** Don't leak credentials in error messages
5. **Log Safely:** Redact sensitive data from logs

### Performance

1. **Minimize Init() Work:** Fast startup is critical
2. **Connection Pooling:** Reuse HTTP clients and connections
3. **Efficient Polling:** Use appropriate polling intervals
4. **Cache When Possible:** Cache provider metadata
5. **Fail Fast:** Return errors quickly for invalid credentials

### Reliability

1. **Handle Nil Gracefully:** Check for nil maps and slices
2. **Provide Defaults:** Use sensible defaults for optional fields
3. **Retry Transient Errors:** Implement exponential backoff
4. **Graceful Degradation:** Continue working if non-critical features fail

### Maintainability

1. **Document Public APIs:** Use godoc comments
2. **Version Your Plugin:** Include semantic versioning
3. **Test Thoroughly:** Unit tests for all methods
4. **Provide Examples:** Include configuration examples

## Testing

### Unit Tests

```go
package main

import (
    "testing"

    "github.com/Wikid82/charon/backend/pkg/dnsprovider"
    "github.com/stretchr/testify/assert"
)

func TestValidateCredentials(t *testing.T) {
    provider := &MyProvider{}

    tests := []struct {
        name      string
        creds     map[string]string
        expectErr bool
    }{
        {
            name:      "valid credentials",
            creds:     map[string]string{"api_key": "test-key"},
            expectErr: false,
        },
        {
            name:      "missing api_key",
            creds:     map[string]string{},
            expectErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := provider.ValidateCredentials(tt.creds)
            if tt.expectErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestMetadata(t *testing.T) {
    provider := &MyProvider{}
    meta := provider.Metadata()

    assert.Equal(t, "myprovider", meta.Type)
    assert.NotEmpty(t, meta.Name)
    assert.False(t, meta.IsBuiltIn)
    assert.Equal(t, dnsprovider.InterfaceVersion, meta.InterfaceVersion)
}
```

### Integration Tests

```go
func TestRealAPIConnection(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    provider := &MyProvider{}
    creds := map[string]string{
        "api_key": os.Getenv("TEST_API_KEY"),
    }

    err := provider.TestCredentials(creds)
    assert.NoError(t, err)
}
```

Run integration tests:

```bash
go test -v ./... -count=1
```

## Troubleshooting

### Common Build Errors

#### `plugin was built with a different version of package`

**Cause:** Dependency version mismatch

**Solution:**

```bash
go clean -cache
go mod tidy
go build -buildmode=plugin
```

#### `cannot use -buildmode=plugin`

**Cause:** CGO not enabled

**Solution:**

```bash
export CGO_ENABLED=1
```

#### `undefined: dnsprovider.ProviderPlugin`

**Cause:** Missing or incorrect import

**Solution:**

```go
import "github.com/Wikid82/charon/backend/pkg/dnsprovider"
```

### Runtime Errors

#### `plugin was built with a different version of Go`

**Cause:** Go version mismatch between plugin and Charon

**Solution:** Rebuild plugin with matching Go version

#### `symbol not found: Plugin`

**Cause:** Plugin variable not exported

**Solution:**

```go
// Must be exported (capitalized)
var Plugin dnsprovider.ProviderPlugin = &MyProvider{}
```

#### `interface version mismatch`

**Cause:** Plugin built against incompatible interface

**Solution:** Update plugin to match Charon's interface version

## Publishing Plugins

### Release Checklist

- [ ] All methods implemented and tested
- [ ] Go version matches current Charon release
- [ ] Interface version set correctly
- [ ] Documentation includes usage examples
- [ ] README includes installation instructions
- [ ] LICENSE file included
- [ ] Changelog maintained
- [ ] GitHub releases with binaries for all platforms

### Distribution

1. **GitHub Releases:**

   ```bash
   # Tag release
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0

   # Build for multiple platforms
   make build-all

   # Create GitHub release and attach binaries
   ```

2. **Signature File:**

   ```bash
   sha256sum *.so > SHA256SUMS
   gpg --sign SHA256SUMS
   ```

3. **Documentation:**
   - Include README with installation instructions
   - Provide configuration examples
   - List required Charon version
   - Include troubleshooting section

## Resources

### Reference Implementation

- **PowerDNS Plugin:** [`plugins/powerdns/main.go`](../../plugins/powerdns/main.go)
- **Built-in Providers:** [`backend/pkg/dnsprovider/builtin/`](../../backend/pkg/dnsprovider/builtin/)
- **Plugin Interface:** [`backend/pkg/dnsprovider/plugin.go`](../../backend/pkg/dnsprovider/plugin.go)

### External Documentation

- [Go Plugin Package](https://pkg.go.dev/plugin)
- [Caddy DNS Providers](https://github.com/caddy-dns)
- [ACME DNS-01 Challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)

### Community

- **GitHub Discussions:** <https://github.com/Wikid82/charon/discussions>
- **Plugin Registry:** <https://github.com/Wikid82/charon-plugins>
- **Issue Tracker:** <https://github.com/Wikid82/charon/issues>

## See Also

- [Custom Plugin Installation Guide](../features/custom-plugins.md)
- [DNS Provider Configuration](../features/dns-providers.md)
- [Contributing Guidelines](../../CONTRIBUTING.md)
