# Phase 5: Custom DNS Provider Plugins - Implementation Specification

**Status:** 📋 Planning
**Priority:** P3 (Lowest)
**Estimated Time:** 24-32 hours
**Author:** Engineering Director
**Date:** January 6, 2026

---

## 1. Executive Summary

This document specifies the implementation of a custom DNS provider plugin system for Charon. This feature enables users to integrate DNS providers not supported out-of-the-box by creating Go plugins that implement a standard interface.

### Key Goals

- Enable extensibility for custom/internal DNS providers
- Maintain backward compatibility with existing providers
- Provide security controls (signature verification, allowlisting)
- Support community-contributed plugins

### Critical Platform Limitation

> ⚠️ **IMPORTANT: Go plugins only work on Linux and macOS.**
>
> The Go `plugin` package does not support Windows. Users on Windows cannot load custom plugins. Built-in providers will continue to work on all platforms.

### Critical Technical Limitations

> ⚠️ **SUPERVISOR REVIEW FINDINGS:** The following limitations must be understood before implementation:

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| **Go Version Match** | Plugin and host must use identical Go version | Document required Go version, verify at load time |
| **Dependency Version Match** | All shared dependencies must match exactly | Use dependency injection, minimize shared deps |
| **No Hot Reload** | Plugins cannot be unloaded from memory | Require restart for plugin updates |
| **CGO Required** | Plugins must be built with `CGO_ENABLED=1` | Document build requirements |
| **Caddy Module Required** | Plugins only handle UI/API - Caddy needs its own DNS module | Document Caddy-side requirements |

### Caddy DNS Module Dependency

External plugins provide:

- UI credential field definitions
- Credential validation
- Caddy config generation

But **Caddy itself must have the matching DNS provider module compiled in**. For example, to use PowerDNS:

1. Install Charon's PowerDNS plugin (this feature) - handles UI/API/credentials
2. Use Caddy built with [caddy-dns/powerdns](https://github.com/caddy-dns/powerdns) - handles actual DNS challenge

---

## 2. Architecture Overview

### 2.1 High-Level Design

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Charon Backend                               │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Provider Registry                         │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │    │
│  │  │  Cloudflare  │  │   Route53    │  │ DigitalOcean │  ...  │    │
│  │  │  (built-in)  │  │  (built-in)  │  │  (built-in)  │       │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘       │    │
│  │  ┌──────────────┐  ┌──────────────┐                         │    │
│  │  │   PowerDNS   │  │   Infoblox   │  ← External Plugins     │    │
│  │  │   (plugin)   │  │   (plugin)   │                         │    │
│  │  └──────────────┘  └──────────────┘                         │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              ▲                                       │
│                              │                                       │
│  ┌───────────────────────────┴─────────────────────────────────┐    │
│  │                     Plugin Loader                            │    │
│  │  - Load .so files from plugins/ directory                    │    │
│  │  - Verify signatures against allowlist                       │    │
│  │  - Register valid plugins with registry                      │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              ▲                                       │
│                              │                                       │
│  ┌───────────────────────────┴─────────────────────────────────┐    │
│  │                 DNS Provider Service                         │    │
│  │  - Queries registry for provider types                       │    │
│  │  - Validates credentials via provider interface              │    │
│  │  - Builds Caddy config via provider interface                │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Directory Structure

```
backend/
├── pkg/
│   └── dnsprovider/
│       ├── plugin.go           # Plugin interface definition
│       ├── registry.go         # Provider registry
│       ├── errors.go           # Plugin-specific errors
│       └── builtin/
│           ├── cloudflare.go   # Built-in Cloudflare provider
│           ├── route53.go      # Built-in Route53 provider
│           ├── digitalocean.go # Built-in DigitalOcean provider
│           ├── googleclouddns.go
│           ├── azure.go
│           ├── namecheap.go
│           ├── godaddy.go
│           ├── hetzner.go
│           ├── vultr.go
│           └── dnsimple.go
├── internal/
│   ├── models/
│   │   └── plugin.go           # Plugin database model
│   ├── services/
│   │   └── plugin_loader.go    # Plugin loading service
│   └── api/
│       └── handlers/
│           └── plugin_handler.go # Plugin API handlers

plugins/                        # External plugins directory
├── powerdns/
│   ├── main.go                 # PowerDNS plugin source
│   └── README.md
└── powerdns.so                 # Compiled plugin

config/
└── plugins.yaml                # Plugin allowlist configuration
```

---

## 3. Backend Implementation

### 3.1 Plugin Interface

**File: `backend/pkg/dnsprovider/plugin.go`**

> ⚠️ **SUPERVISOR CORRECTIONS APPLIED:** Added lifecycle hooks (Init/Cleanup), multi-credential support, and version compatibility fields per Supervisor review.

```go
package dnsprovider

import "time"

// InterfaceVersion is the current plugin interface version.
// Plugins built against a different version may not be compatible.
const InterfaceVersion = "v1"

// ProviderPlugin defines the interface that all DNS provider plugins must implement.
// Both built-in providers and external plugins implement this interface.
type ProviderPlugin interface {
    // Type returns the unique provider type identifier (e.g., "cloudflare", "powerdns")
    // This must be lowercase, alphanumeric with optional underscores.
    Type() string

    // Metadata returns descriptive information about the provider for UI display.
    Metadata() ProviderMetadata

    // Init is called after the plugin is registered. Use for startup initialization
    // (loading config files, validating environment, establishing connections).
    // Return an error to prevent the plugin from being registered.
    Init() error

    // Cleanup is called before the plugin is unregistered. Use for resource cleanup
    // (closing connections, flushing caches). Note: Go plugins cannot be unloaded
    // from memory - this is only called during graceful shutdown.
    Cleanup() error

    // RequiredCredentialFields returns the credential fields that must be provided.
    RequiredCredentialFields() []CredentialFieldSpec

    // OptionalCredentialFields returns credential fields that may be provided.
    OptionalCredentialFields() []CredentialFieldSpec

    // ValidateCredentials checks if the provided credentials are valid.
    // Returns nil if valid, error describing the issue otherwise.
    ValidateCredentials(creds map[string]string) error

    // TestCredentials attempts to verify credentials work with the provider API.
    // This may make network calls to the provider.
    TestCredentials(creds map[string]string) error

    // SupportsMultiCredential indicates if the provider can handle zone-specific credentials.
    // If true, BuildCaddyConfigForZone will be called instead of BuildCaddyConfig when
    // multi-credential mode is enabled (Phase 3 feature).
    SupportsMultiCredential() bool

    // BuildCaddyConfig constructs the Caddy DNS challenge configuration.
    // The returned map is embedded into Caddy's TLS automation policy.
    // Used when multi-credential mode is disabled.
    BuildCaddyConfig(creds map[string]string) map[string]any

    // BuildCaddyConfigForZone constructs config for a specific zone (multi-credential mode).
    // Only called if SupportsMultiCredential() returns true.
    // baseDomain is the zone being configured (e.g., "example.com").
    BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any

    // PropagationTimeout returns the recommended DNS propagation wait time.
    PropagationTimeout() time.Duration

    // PollingInterval returns the recommended polling interval for DNS verification.
    PollingInterval() time.Duration
}

// ProviderMetadata contains descriptive information about a DNS provider.
type ProviderMetadata struct {
    Type             string `json:"type"`
    Name             string `json:"name"`
    Description      string `json:"description"`
    DocumentationURL string `json:"documentation_url,omitempty"`
    Author           string `json:"author,omitempty"`
    Version          string `json:"version,omitempty"`
    IsBuiltIn        bool   `json:"is_built_in"`

    // Version compatibility (required for external plugins)
    GoVersion        string `json:"go_version,omitempty"`        // Go version used to build (e.g., "1.23.4")
    InterfaceVersion string `json:"interface_version,omitempty"` // Plugin interface version (e.g., "v1")
}

// CredentialFieldSpec defines a credential field for UI rendering.
type CredentialFieldSpec struct {
    Name        string `json:"name"`        // Field key (e.g., "api_token")
    Label       string `json:"label"`       // Display label (e.g., "API Token")
    Type        string `json:"type"`        // "text", "password", "textarea", "select"
    Placeholder string `json:"placeholder"` // Input placeholder text
    Hint        string `json:"hint"`        // Help text shown below field
    Options     []SelectOption `json:"options,omitempty"` // For "select" type
}

// SelectOption represents an option in a select dropdown.
type SelectOption struct {
    Value string `json:"value"`
    Label string `json:"label"`
}
```

### 3.2 Provider Registry

**File: `backend/pkg/dnsprovider/registry.go`**

```go
package dnsprovider

import (
    "fmt"
    "sync"
)

// Registry is a thread-safe registry of DNS provider plugins.
type Registry struct {
    providers map[string]ProviderPlugin
    mu        sync.RWMutex
}

// globalRegistry is the singleton registry instance.
var globalRegistry = &Registry{
    providers: make(map[string]ProviderPlugin),
}

// Global returns the global provider registry.
func Global() *Registry {
    return globalRegistry
}

// Register adds a provider to the registry.
func (r *Registry) Register(provider ProviderPlugin) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    providerType := provider.Type()
    if providerType == "" {
        return fmt.Errorf("provider type cannot be empty")
    }

    if _, exists := r.providers[providerType]; exists {
        return fmt.Errorf("provider type %q already registered", providerType)
    }

    r.providers[providerType] = provider
    return nil
}

// Get retrieves a provider by type.
func (r *Registry) Get(providerType string) (ProviderPlugin, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    provider, ok := r.providers[providerType]
    return provider, ok
}

// List returns all registered providers.
func (r *Registry) List() []ProviderPlugin {
    r.mu.RLock()
    defer r.mu.RUnlock()

    providers := make([]ProviderPlugin, 0, len(r.providers))
    for _, p := range r.providers {
        providers = append(providers, p)
    }
    return providers
}

// Types returns all registered provider type identifiers.
func (r *Registry) Types() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()

    types := make([]string, 0, len(r.providers))
    for t := range r.providers {
        types = append(types, t)
    }
    return types
}

// IsSupported checks if a provider type is registered.
func (r *Registry) IsSupported(providerType string) bool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    _, ok := r.providers[providerType]
    return ok
}

// Unregister removes a provider from the registry.
// Used primarily for plugin unloading.
func (r *Registry) Unregister(providerType string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.providers, providerType)
}
```

### 3.3 Built-in Provider Example

**File: `backend/pkg/dnsprovider/builtin/cloudflare.go`**

```go
package builtin

import (
    "fmt"
    "time"

    "github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// CloudflareProvider implements the ProviderPlugin interface for Cloudflare DNS.
type CloudflareProvider struct{}

func init() {
    dnsprovider.Global().Register(&CloudflareProvider{})
}

func (p *CloudflareProvider) Type() string {
    return "cloudflare"
}

func (p *CloudflareProvider) Metadata() dnsprovider.ProviderMetadata {
    return dnsprovider.ProviderMetadata{
        Type:             "cloudflare",
        Name:             "Cloudflare",
        Description:      "Cloudflare DNS with API Token authentication",
        DocumentationURL: "https://developers.cloudflare.com/api/tokens/create/",
        IsBuiltIn:        true,
    }
}

func (p *CloudflareProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
    return []dnsprovider.CredentialFieldSpec{
        {
            Name:        "api_token",
            Label:       "API Token",
            Type:        "password",
            Placeholder: "Enter your Cloudflare API token",
            Hint:        "Token requires Zone:DNS:Edit permission",
        },
    }
}

func (p *CloudflareProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
    return []dnsprovider.CredentialFieldSpec{
        {
            Name:        "zone_id",
            Label:       "Zone ID",
            Type:        "text",
            Placeholder: "Optional: Specific zone ID",
            Hint:        "Leave empty to auto-detect zone",
        },
    }
}

func (p *CloudflareProvider) ValidateCredentials(creds map[string]string) error {
    if creds["api_token"] == "" {
        return fmt.Errorf("api_token is required")
    }
    return nil
}

func (p *CloudflareProvider) TestCredentials(creds map[string]string) error {
    // Implementation would make API call to verify token
    // For now, just validate required fields
    return p.ValidateCredentials(creds)
}

func (p *CloudflareProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
    config := map[string]any{
        "name":      "cloudflare",
        "api_token": creds["api_token"],
    }
    if zoneID := creds["zone_id"]; zoneID != "" {
        config["zone_id"] = zoneID
    }
    return config
}

func (p *CloudflareProvider) PropagationTimeout() time.Duration {
    return 120 * time.Second
}

func (p *CloudflareProvider) PollingInterval() time.Duration {
    return 5 * time.Second
}
```

### 3.4 Plugin Loader Service

**File: `backend/internal/services/plugin_loader.go`**

```go
package services

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "os"
    "path/filepath"
    "plugin"
    "strings"
    "sync"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// PluginLoaderService manages loading and unloading of DNS provider plugins.
type PluginLoaderService struct {
    pluginDir     string
    allowedSigs   map[string]string // plugin name -> expected signature
    loadedPlugins map[string]string // plugin type -> file path
    mu            sync.RWMutex
}

// NewPluginLoaderService creates a new plugin loader.
func NewPluginLoaderService(pluginDir string, allowedSignatures map[string]string) *PluginLoaderService {
    return &PluginLoaderService{
        pluginDir:     pluginDir,
        allowedSigs:   allowedSignatures,
        loadedPlugins: make(map[string]string),
    }
}

// LoadAllPlugins loads all .so files from the plugin directory.
func (s *PluginLoaderService) LoadAllPlugins() error {
    if s.pluginDir == "" {
        logger.Log().Info("Plugin directory not configured, skipping plugin loading")
        return nil
    }

    entries, err := os.ReadDir(s.pluginDir)
    if err != nil {
        if os.IsNotExist(err) {
            logger.Log().Info("Plugin directory does not exist, skipping plugin loading")
            return nil
        }
        return fmt.Errorf("failed to read plugin directory: %w", err)
    }

    loadedCount := 0
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".so") {
            continue
        }

        pluginPath := filepath.Join(s.pluginDir, entry.Name())
        if err := s.LoadPlugin(pluginPath); err != nil {
            logger.Log().WithError(err).Warnf("Failed to load plugin: %s", entry.Name())
            continue
        }
        loadedCount++
    }

    logger.Log().Infof("Loaded %d external DNS provider plugins", loadedCount)
    return nil
}

// LoadPlugin loads a single plugin from the specified path.
func (s *PluginLoaderService) LoadPlugin(path string) error {
    // Verify signature if signatures are configured
    if len(s.allowedSigs) > 0 {
        pluginName := strings.TrimSuffix(filepath.Base(path), ".so")
        expectedSig, ok := s.allowedSigs[pluginName]
        if !ok {
            return fmt.Errorf("plugin %q not in allowlist", pluginName)
        }

        actualSig, err := s.computeSignature(path)
        if err != nil {
            return fmt.Errorf("failed to compute signature: %w", err)
        }

        if actualSig != expectedSig {
            return fmt.Errorf("signature mismatch for %q: expected %s, got %s", pluginName, expectedSig, actualSig)
        }
    }

    // Load the plugin
    p, err := plugin.Open(path)
    if err != nil {
        return fmt.Errorf("failed to open plugin: %w", err)
    }

    // Look up the Plugin symbol
    symbol, err := p.Lookup("Plugin")
    if err != nil {
        return fmt.Errorf("plugin missing 'Plugin' symbol: %w", err)
    }

    // Assert the interface
    provider, ok := symbol.(dnsprovider.ProviderPlugin)
    if !ok {
        // Try pointer to interface
        providerPtr, ok := symbol.(*dnsprovider.ProviderPlugin)
        if !ok {
            return fmt.Errorf("'Plugin' symbol does not implement ProviderPlugin interface")
        }
        provider = *providerPtr
    }

    // Validate provider
    meta := provider.Metadata()
    if meta.Type == "" || meta.Name == "" {
        return fmt.Errorf("plugin has invalid metadata")
    }

    // Register with global registry
    if err := dnsprovider.Global().Register(provider); err != nil {
        return fmt.Errorf("failed to register plugin: %w", err)
    }

    s.mu.Lock()
    s.loadedPlugins[provider.Type()] = path
    s.mu.Unlock()

    logger.Log().WithFields(map[string]interface{}{
        "type":    meta.Type,
        "name":    meta.Name,
        "version": meta.Version,
        "author":  meta.Author,
    }).Info("Loaded DNS provider plugin")

    return nil
}

// computeSignature calculates SHA-256 hash of plugin file.
func (s *PluginLoaderService) computeSignature(path string) (string, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    hash := sha256.Sum256(data)
    return "sha256:" + hex.EncodeToString(hash[:]), nil
}

// ListLoadedPlugins returns information about loaded external plugins.
func (s *PluginLoaderService) ListLoadedPlugins() []dnsprovider.ProviderMetadata {
    s.mu.RLock()
    defer s.mu.RUnlock()

    var plugins []dnsprovider.ProviderMetadata
    for providerType := range s.loadedPlugins {
        if provider, ok := dnsprovider.Global().Get(providerType); ok {
            plugins = append(plugins, provider.Metadata())
        }
    }
    return plugins
}

// IsPluginLoaded checks if a provider type was loaded from an external plugin.
func (s *PluginLoaderService) IsPluginLoaded(providerType string) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    _, ok := s.loadedPlugins[providerType]
    return ok
}
```

### 3.5 Plugin Database Model

**File: `backend/internal/models/plugin.go`**

```go
package models

import "time"

// Plugin represents an installed DNS provider plugin.
type Plugin struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    UUID      string    `json:"uuid" gorm:"uniqueIndex;size:36"`
    Name      string    `json:"name" gorm:"not null;size:255"`
    Type      string    `json:"type" gorm:"uniqueIndex;not null;size:100"`
    FilePath  string    `json:"file_path" gorm:"not null;size:500"`
    Signature string    `json:"signature" gorm:"size:100"`
    Enabled   bool      `json:"enabled" gorm:"default:true"`
    Status    string    `json:"status" gorm:"default:'pending';size:50"` // pending, loaded, error
    Error     string    `json:"error,omitempty" gorm:"type:text"`
    Version   string    `json:"version,omitempty" gorm:"size:50"`
    Author    string    `json:"author,omitempty" gorm:"size:255"`
    LoadedAt  *time.Time `json:"loaded_at,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (Plugin) TableName() string {
    return "plugins"
}
```

### 3.6 Integration Changes

**Modify: `backend/internal/services/dns_provider_service.go`**

Replace hardcoded `SupportedProviderTypes` with registry query:

```go
// Before (hardcoded):
// var SupportedProviderTypes = []string{"cloudflare", "route53", ...}

// After (registry-based):
func (s *dnsProviderService) GetSupportedProviderTypes() []string {
    return dnsprovider.Global().Types()
}

func (s *dnsProviderService) IsProviderTypeSupported(providerType string) bool {
    return dnsprovider.Global().IsSupported(providerType)
}

func (s *dnsProviderService) GetProviderCredentialFields(providerType string) ([]dnsprovider.CredentialFieldSpec, error) {
    provider, ok := dnsprovider.Global().Get(providerType)
    if !ok {
        return nil, fmt.Errorf("unsupported provider type: %s", providerType)
    }

    fields := provider.RequiredCredentialFields()
    fields = append(fields, provider.OptionalCredentialFields()...)
    return fields, nil
}
```

**Modify: `backend/internal/caddy/config.go`**

Replace `buildDNSChallengeIssuer` to use registry:

```go
func (b *ConfigBuilder) buildDNSChallengeIssuer(dnsConfig *DNSProviderConfig) map[string]any {
    provider, ok := dnsprovider.Global().Get(dnsConfig.ProviderType)
    if !ok {
        logger.Log().Warnf("Unknown provider type: %s", dnsConfig.ProviderType)
        return nil
    }

    // Build Caddy config using provider interface
    providerConfig := provider.BuildCaddyConfig(dnsConfig.Credentials)

    return map[string]any{
        "module": "acme",
        "challenges": map[string]any{
            "dns": map[string]any{
                "provider": providerConfig,
                "propagation_timeout": int(provider.PropagationTimeout().Seconds()),
            },
        },
    }
}
```

---

## 4. Frontend Implementation

### 4.1 Plugin Management Page

**File: `frontend/src/pages/Plugins.tsx`**

Features:

- List all installed plugins (built-in and external)
- Show status (loaded, error, disabled)
- Enable/disable toggle for external plugins
- View plugin metadata and error details
- Refresh button to reload plugins

### 4.2 Dynamic Credential Fields

**Modify: `frontend/src/components/DNSProviderForm.tsx`**

Query provider credential fields from API instead of hardcoded mapping:

```typescript
// New API endpoint: GET /api/v1/dns-providers/types/:type/fields
const { data: credentialFields } = useProviderCredentialFields(providerType);

// Render fields dynamically
{credentialFields?.map(field => (
    <FormField
        key={field.name}
        name={field.name}
        label={field.label}
        type={field.type}
        placeholder={field.placeholder}
        hint={field.hint}
        required={/* check if in required fields */}
    />
))}
```

### 4.3 API Client

**File: `frontend/src/api/plugins.ts`**

```typescript
export interface PluginInfo {
    id: number;
    uuid: string;
    name: string;
    type: string;
    enabled: boolean;
    status: 'pending' | 'loaded' | 'error';
    error?: string;
    version?: string;
    author?: string;
    is_built_in: boolean;
    loaded_at?: string;
}

export const pluginsApi = {
    list: () => api.get<PluginInfo[]>('/api/v1/admin/plugins'),
    enable: (id: number) => api.post(`/api/v1/admin/plugins/${id}/enable`),
    disable: (id: number) => api.post(`/api/v1/admin/plugins/${id}/disable`),
    reload: () => api.post('/api/v1/admin/plugins/reload'),
};
```

---

## 5. Example Plugin: PowerDNS

**File: `plugins/powerdns/main.go`**

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// Plugin is the exported symbol that Charon looks for.
var Plugin dnsprovider.ProviderPlugin = &PowerDNSProvider{}

type PowerDNSProvider struct{}

func (p *PowerDNSProvider) Type() string {
    return "powerdns"
}

func (p *PowerDNSProvider) Metadata() dnsprovider.ProviderMetadata {
    return dnsprovider.ProviderMetadata{
        Type:             "powerdns",
        Name:             "PowerDNS",
        Description:      "PowerDNS Authoritative Server with HTTP API",
        DocumentationURL: "https://doc.powerdns.com/authoritative/http-api/",
        Author:           "Charon Community",
        Version:          "1.0.0",
        IsBuiltIn:        false,
    }
}

func (p *PowerDNSProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
    return []dnsprovider.CredentialFieldSpec{
        {
            Name:        "api_url",
            Label:       "API URL",
            Type:        "text",
            Placeholder: "https://pdns.example.com:8081",
            Hint:        "PowerDNS HTTP API endpoint",
        },
        {
            Name:        "api_key",
            Label:       "API Key",
            Type:        "password",
            Placeholder: "Your PowerDNS API key",
            Hint:        "X-API-Key header value",
        },
    }
}

func (p *PowerDNSProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
    return []dnsprovider.CredentialFieldSpec{
        {
            Name:        "server_id",
            Label:       "Server ID",
            Type:        "text",
            Placeholder: "localhost",
            Hint:        "PowerDNS server ID (default: localhost)",
        },
    }
}

func (p *PowerDNSProvider) ValidateCredentials(creds map[string]string) error {
    if creds["api_url"] == "" {
        return fmt.Errorf("api_url is required")
    }
    if creds["api_key"] == "" {
        return fmt.Errorf("api_key is required")
    }
    return nil
}

func (p *PowerDNSProvider) TestCredentials(creds map[string]string) error {
    if err := p.ValidateCredentials(creds); err != nil {
        return err
    }

    // Test API connectivity
    serverID := creds["server_id"]
    if serverID == "" {
        serverID = "localhost"
    }

    url := fmt.Sprintf("%s/api/v1/servers/%s", creds["api_url"], serverID)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
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

func (p *PowerDNSProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
    serverID := creds["server_id"]
    if serverID == "" {
        serverID = "localhost"
    }

    return map[string]any{
        "name":      "powerdns",
        "api_url":   creds["api_url"],
        "api_key":   creds["api_key"],
        "server_id": serverID,
    }
}

func (p *PowerDNSProvider) PropagationTimeout() time.Duration {
    return 60 * time.Second
}

func (p *PowerDNSProvider) PollingInterval() time.Duration {
    return 2 * time.Second
}
```

**Build Command:**

```bash
cd plugins/powerdns
go build -buildmode=plugin -o ../powerdns.so main.go
```

---

## 6. Security

> ⚠️ **SUPERVISOR CORRECTIONS APPLIED:** Enhanced security section with directory permissions, TOCTOU mitigation, and critical warnings.

### 6.1 Critical Security Warnings

> 🚨 **IN-PROCESS EXECUTION:** External plugins run in the same process as Charon. A malicious plugin has full access to:
>
> - All process memory (including encryption keys)
> - Database connections
> - Network capabilities
> - Filesystem access
>
> **Only load plugins from trusted sources.** Code review is mandatory.

> 🚨 **PLUGINS CANNOT BE UNLOADED:** Once loaded, plugin code remains in memory until Charon restarts. "Disabling" a plugin only removes it from the registry.

### 6.2 Signature Verification

Plugins are verified using SHA-256 hash of the plugin binary:

```bash
# Generate signature
sha256sum plugins/powerdns.so
# Output: abc123... plugins/powerdns.so

# Configure in config/plugins.yaml
plugins:
  powerdns:
    enabled: true
    signature: "sha256:abc123..."
```

### 6.3 Plugin Allowlist

Only plugins listed in `config/plugins.yaml` are loaded:

```yaml
# config/plugins.yaml
plugins:
  powerdns:
    enabled: true
    signature: "sha256:abc123def456..."
  infoblox:
    enabled: false  # Disabled
    signature: "sha256:789xyz..."
```

### 6.4 Directory Permission Requirements

The plugin directory MUST have restricted permissions:

```bash
# Set secure permissions (Linux/macOS)
chmod 700 /opt/charon/plugins
chown charon:charon /opt/charon/plugins

# Verify permissions before loading (implemented in plugin_loader.go)
# Loader will refuse to load if permissions are too permissive (e.g., world-readable)
```

### 6.5 Security Recommendations

1. **Code Review:** Always review plugin source before deployment
2. **Isolated Builds:** Build plugins in isolated environments
3. **Regular Updates:** Keep plugin signatures updated after rebuilds
4. **Minimal Permissions:** Run Charon with minimal filesystem permissions
5. **Audit Logging:** All plugin load events are logged
6. **Version Pinning:** Pin Go version and dependencies in plugin builds
7. **Separate Build Environment:** Never build plugins on production systems

---

## 7. Configuration

### 7.1 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CHARON_PLUGINS_ENABLED` | Enable plugin system | `false` |
| `CHARON_PLUGINS_DIR` | Plugin directory path | `/app/plugins` |
| `CHARON_PLUGINS_CONFIG` | Plugin allowlist file | `/app/config/plugins.yaml` |
| `CHARON_PLUGINS_STRICT_MODE` | Reject unsigned plugins entirely | `true` |

### 7.2 Example Configuration

```bash
# Enable plugins
export CHARON_PLUGINS_ENABLED=true
export CHARON_PLUGINS_DIR=/opt/charon/plugins
export CHARON_PLUGINS_CONFIG=/opt/charon/config/plugins.yaml
export CHARON_PLUGINS_STRICT_MODE=true
```

### 7.3 Build Requirements

> ⚠️ **CGO Required:** Go plugins require CGO. Build plugins with:
>
> ```bash
> CGO_ENABLED=1 go build -buildmode=plugin -o plugin.so main.go
> ```

---

## 8. API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/admin/plugins` | List all plugins (built-in + external) |
| `GET` | `/api/v1/admin/plugins/:id` | Get plugin details |
| `POST` | `/api/v1/admin/plugins/:id/enable` | Enable a plugin |
| `POST` | `/api/v1/admin/plugins/:id/disable` | Disable a plugin |
| `POST` | `/api/v1/admin/plugins/reload` | Reload all plugins |
| `GET` | `/api/v1/dns-providers/types` | List available provider types |
| `GET` | `/api/v1/dns-providers/types/:type/fields` | Get credential fields for type |

---

## 9. Testing Strategy

### 9.1 Unit Tests

- Plugin interface compliance tests
- Registry add/remove/query tests
- Plugin loader signature verification tests
- Built-in provider tests

### 9.2 Integration Tests

- Load example plugin and verify registration
- Create DNS provider with plugin type
- Build Caddy config with plugin provider

### 9.3 Coverage Target

- Backend: ≥85% coverage
- Frontend: ≥85% coverage

---

## 10. Files to Create

| File | Description | Est. Lines |
|------|-------------|------------|
| `backend/pkg/dnsprovider/plugin.go` | Plugin interface | 100 |
| `backend/pkg/dnsprovider/registry.go` | Provider registry | 120 |
| `backend/pkg/dnsprovider/errors.go` | Plugin errors | 30 |
| `backend/pkg/dnsprovider/builtin/cloudflare.go` | Cloudflare provider | 80 |
| `backend/pkg/dnsprovider/builtin/route53.go` | Route53 provider | 100 |
| `backend/pkg/dnsprovider/builtin/digitalocean.go` | DigitalOcean provider | 80 |
| `backend/pkg/dnsprovider/builtin/googleclouddns.go` | Google Cloud DNS | 100 |
| `backend/pkg/dnsprovider/builtin/azure.go` | Azure DNS | 100 |
| `backend/pkg/dnsprovider/builtin/namecheap.go` | Namecheap | 80 |
| `backend/pkg/dnsprovider/builtin/godaddy.go` | GoDaddy | 80 |
| `backend/pkg/dnsprovider/builtin/hetzner.go` | Hetzner | 80 |
| `backend/pkg/dnsprovider/builtin/vultr.go` | Vultr | 80 |
| `backend/pkg/dnsprovider/builtin/dnsimple.go` | DNSimple | 80 |
| `backend/pkg/dnsprovider/builtin/init.go` | Auto-register built-ins | 20 |
| `backend/internal/models/plugin.go` | Plugin model | 40 |
| `backend/internal/services/plugin_loader.go` | Plugin loader | 200 |
| `backend/internal/api/handlers/plugin_handler.go` | Plugin API | 150 |
| `plugins/powerdns/main.go` | Example plugin | 150 |
| `frontend/src/pages/Plugins.tsx` | Plugin admin page | 250 |
| `frontend/src/api/plugins.ts` | Plugin API client | 60 |
| `frontend/src/hooks/usePlugins.ts` | Plugin hooks | 50 |
| `docs/features/custom-plugins.md` | User guide | 300 |
| `docs/development/plugin-development.md` | Developer guide | 500 |

**Total New Code:** ~2,830 lines

---

## 11. Files to Modify

| File | Changes |
|------|---------|
| `backend/internal/services/dns_provider_service.go` | Use registry instead of hardcoded lists |
| `backend/internal/caddy/config.go` | Use registry for Caddy config building |
| `backend/main.go` | Initialize plugin system on startup |
| `backend/internal/api/routes.go` | Register plugin API routes |
| `backend/internal/database/database.go` | AutoMigrate Plugin model |
| `frontend/src/components/DNSProviderForm.tsx` | Dynamic credential fields |
| `frontend/src/App.tsx` | Add Plugins route |
| `frontend/src/components/Sidebar.tsx` | Add Plugins link |

---

## 12. Implementation Order

| Phase | Task | Est. Hours |
|-------|------|------------|
| 1 | Plugin interface and registry | 2 |
| 2 | Migrate built-in providers (10 providers) | 5 |
| 3 | Plugin loader with signature verification | 3 |
| 4 | Plugin model and database migration | 1 |
| 5 | Plugin API handlers | 2 |
| 6 | Modify dns_provider_service for registry | 2 |
| 7 | Modify Caddy config for registry | 2 |
| 8 | Frontend plugin management page | 4 |
| 9 | Dynamic credential fields in UI | 3 |
| 10 | PowerDNS example plugin | 2 |
| 11 | Unit tests (85% coverage) | 4 |
| 12 | Documentation | 2 |

**Total: 32 hours**

---

## 13. Rollback Plan

1. **Plugin Load Failure:** Log error, continue without plugin, show error in admin UI
2. **Registry Failure:** Fall back to empty registry, built-in providers still work via init()
3. **Complete Disable:** Set `CHARON_PLUGINS_ENABLED=false` to disable entire plugin system

---

## 14. Definition of Done

- [ ] Plugin interface defined (`backend/pkg/dnsprovider/plugin.go`)
- [ ] Provider registry implemented (`backend/pkg/dnsprovider/registry.go`)
- [ ] All 10 built-in providers migrated to registry
- [ ] Plugin loader with signature verification
- [ ] Plugin database model and migration
- [ ] Plugin CRUD API endpoints
- [ ] DNS provider service uses registry
- [ ] Caddy config builder uses registry
- [ ] Frontend plugin management page
- [ ] Dynamic credential fields in DNSProviderForm
- [ ] PowerDNS example plugin compiles and loads
- [ ] Backend tests ≥85% coverage
- [ ] Frontend tests ≥85% coverage
- [ ] User documentation
- [ ] Developer guide
- [ ] Security scans pass
- [ ] Pre-commit hooks pass

---

## 15. Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-01-06 | Engineering Director | Initial specification |
