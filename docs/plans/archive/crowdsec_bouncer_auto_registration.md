# CrowdSec Bouncer Auto-Registration and Key Persistence

**Status**: Ready for Implementation
**Priority**: P1 (User Experience Enhancement)
**Created**: 2026-02-03
**Estimated Effort**: 8-12 hours

---

## Executive Summary

CrowdSec bouncer integration currently requires manual key generation and configuration, which is error-prone and leads to "access forbidden" errors when users set invented (non-registered) keys. This plan implements automatic bouncer registration with persistent key storage, eliminating manual key management.

### Goals

1. **Zero-config CrowdSec**: Users enable CrowdSec via GUI toggle without touching API keys
2. **Self-healing**: System auto-registers and stores valid keys, recovers from invalid state
3. **Transparency**: Users can view their bouncer key in the UI if needed for external tools
4. **Backward Compatibility**: Existing env-var overrides continue to work

---

## Current State Analysis

### How It Works Today

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Current Flow (Manual, Error-Prone)                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. User sets docker-compose.yml:                                            │
│     CHARON_SECURITY_CROWDSEC_API_KEY=invented-key-that-doesnt-work           │
│     ❌ PROBLEM: Key is NOT registered with CrowdSec                          │
│                                                                              │
│  2. Container starts:                                                        │
│     docker-entrypoint.sh runs:                                               │
│     └─► cscli machines add -a --force (machine only, NOT bouncer)           │
│                                                                              │
│  3. User enables CrowdSec via GUI:                                           │
│     POST /api/v1/admin/crowdsec/start                                        │
│     └─► Start crowdsec process                                              │
│     └─► Wait for LAPI ready                                                 │
│     └─► ❌ NO bouncer registration                                           │
│                                                                              │
│  4. Caddy config generation:                                                 │
│     getCrowdSecAPIKey() returns "invented-key-that-doesnt-work"              │
│     └─► Reads from CHARON_SECURITY_CROWDSEC_API_KEY env var                 │
│     └─► ❌ No file fallback, no validation                                   │
│                                                                              │
│  5. CrowdSec bouncer connects:                                               │
│     └─► Sends invalid key to LAPI                                           │
│     └─► ❌ "access forbidden" error                                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Files Involved

| File | Purpose | Current Behavior |
|------|---------|------------------|
| [.docker/docker-entrypoint.sh](../../.docker/docker-entrypoint.sh) | Container startup | Registers machine only, not bouncer |
| [configs/crowdsec/register_bouncer.sh](../../configs/crowdsec/register_bouncer.sh) | Bouncer registration script | Exists but never called automatically |
| [backend/internal/caddy/config.go](../../backend/internal/caddy/config.go#L1129) | `getCrowdSecAPIKey()` | Reads env vars only, no file fallback |
| [backend/internal/api/handlers/crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go#L191) | `Start()` handler | Starts CrowdSec but doesn't register bouncer |
| [backend/internal/crowdsec/registration.go](../../backend/internal/crowdsec/registration.go) | Registration utilities | Has helpers but not integrated into startup |

### Current Key Storage Locations

| Location | Used For | Current State |
|----------|----------|---------------|
| `CHARON_SECURITY_CROWDSEC_API_KEY` env var | User-provided override | Only source checked |
| `/etc/crowdsec/bouncers/caddy-bouncer.key` | Generated key file | Written by script but never read |
| `/app/data/crowdsec/` | Persistent volume | Not used for key storage |

---

## Proposed Design

### New Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Proposed Flow (Automatic, Self-Healing)               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. Container starts:                                                        │
│     docker-entrypoint.sh runs:                                               │
│     └─► cscli machines add -a --force                                        │
│     └─► 🆕 mkdir -p /app/data/crowdsec                                       │
│     └─► 🆕 Check for env var key and validate if present                     │
│                                                                              │
│  2. User enables CrowdSec via GUI:                                           │
│     POST /api/v1/admin/crowdsec/start                                        │
│     └─► Start crowdsec process                                              │
│     └─► Wait for LAPI ready                                                 │
│     └─► 🆕 ensureBouncerRegistered()                                         │
│         ├─► Check env var key → if set AND valid → use it                  │
│         ├─► Check file key → if exists AND valid → use it                  │
│         ├─► Otherwise → register new bouncer                                │
│         └─► Save valid key to /app/data/crowdsec/bouncer_key                │
│     └─► 🆕 Log key to container logs (for user reference)                   │
│     └─► 🆕 Regenerate Caddy config with valid key                           │
│                                                                              │
│  3. Caddy config generation:                                                 │
│     getCrowdSecAPIKey() with file fallback:                                  │
│     └─► Check env vars (priority 1)                                         │
│     └─► Check /app/data/crowdsec/bouncer_key (priority 2)                   │
│     └─► Return empty string if neither exists                               │
│                                                                              │
│  4. CrowdSec bouncer connects:                                               │
│     └─► Uses validated/registered key                                       │
│     └─► ✅ Connection successful                                             │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Persistence Location

**Primary location**: `/app/data/crowdsec/bouncer_key`

- Inside the persistent volume mount (`/app/data`)
- Survives container rebuilds
- Protected permissions (600)
- Contains plain text API key

**Symlink for compatibility**: `/etc/crowdsec/bouncers/caddy-bouncer.key` → `/app/data/crowdsec/bouncer_key`

- Maintains compatibility with `register_bouncer.sh` script

### Key Priority Order

When determining which API key to use:

```
┌────────────────────────────────────────────────────────────────┐
│                    API Key Resolution Order                     │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Environment Variables (HIGHEST PRIORITY)                    │
│     ├─► CROWDSEC_API_KEY                                        │
│     ├─► CROWDSEC_BOUNCER_API_KEY                                │
│     ├─► CERBERUS_SECURITY_CROWDSEC_API_KEY                      │
│     ├─► CHARON_SECURITY_CROWDSEC_API_KEY                        │
│     └─► CPM_SECURITY_CROWDSEC_API_KEY                           │
│     IF env var set → validate key → use if valid                │
│     IF env var set but INVALID → log warning → continue search  │
│                                                                 │
│  2. Persistent Key File                                         │
│     /app/data/crowdsec/bouncer_key                              │
│     IF file exists → validate key → use if valid                │
│                                                                 │
│  3. Auto-Registration                                           │
│     IF no valid key found → register new bouncer                │
│     → save to /app/data/crowdsec/bouncer_key                    │
│     → use new key                                               │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

---

## Requirements (EARS Notation)

### Must Have (P0)

#### REQ-1: Auto-Registration on CrowdSec Start

**WHEN** CrowdSec is started via GUI or API and no valid bouncer key exists, **THE SYSTEM SHALL** automatically register a bouncer with CrowdSec LAPI.

**Acceptance Criteria**:

- [ ] Bouncer registration occurs after LAPI is confirmed ready
- [ ] Registration uses `cscli bouncers add caddy-bouncer -o raw`
- [ ] Generated key is captured and stored
- [ ] No user intervention required

#### REQ-2: Key Persistence to Volume

**WHEN** a bouncer key is generated or validated, **THE SYSTEM SHALL** persist it to `/app/data/crowdsec/bouncer_key`.

**Acceptance Criteria**:

- [ ] Key file created with permissions 600
- [ ] Key survives container restart/rebuild
- [ ] Key file contains plain text API key (no JSON wrapper)
- [ ] Key file owned by `charon:charon` user

#### REQ-3: Key Logging for User Reference

**WHEN** a new bouncer key is generated, **THE SYSTEM SHALL** log the key to container logs with clear instructions.

**Acceptance Criteria**:

- [ ] Log message includes full API key
- [ ] Log message explains where key is stored
- [ ] Log message advises user to copy key if using external CrowdSec
- [ ] Log level is INFO (not DEBUG)

**Log Format**:

```text
════════════════════════════════════════════════════════════════════
🔐 CrowdSec Bouncer Registered Successfully
────────────────────────────────────────────────────────────────────
Bouncer Name: caddy-bouncer
API Key:      PNoOaOwrUZgSN9nuYuk9BdnCqpp6xLrdXcZwwCh2GSs
Saved To:     /app/data/crowdsec/bouncer_key
────────────────────────────────────────────────────────────────────
💡 TIP: If connecting to an EXTERNAL CrowdSec instance, copy this
   key to your docker-compose.yml as CHARON_SECURITY_CROWDSEC_API_KEY
════════════════════════════════════════════════════════════════════
```

#### REQ-4: File Fallback for API Key

**WHEN** no API key is provided via environment variable, **THE SYSTEM SHALL** read the key from `/app/data/crowdsec/bouncer_key`.

**Acceptance Criteria**:

- [ ] `getCrowdSecAPIKey()` checks file after env vars
- [ ] Empty/whitespace-only files are treated as no key
- [ ] File read errors are logged but don't crash

#### REQ-5: Environment Variable Priority

**WHEN** both file key and env var key exist, **THE SYSTEM SHALL** prefer the environment variable.

**Acceptance Criteria**:

- [ ] Env var always takes precedence
- [ ] Log message indicates env var override is active
- [ ] File key is still validated for future use

### Should Have (P1)

#### REQ-6: Key Validation on Startup

**WHEN** CrowdSec starts with an existing API key, **THE SYSTEM SHALL** validate the key is registered with LAPI before using it.

**Acceptance Criteria**:

- [ ] Validation uses `cscli bouncers list` to check registration
- [ ] Invalid keys trigger warning log
- [ ] Invalid keys trigger auto-registration

**Validation Flow**:

```text
1. Check if 'caddy-bouncer' exists in cscli bouncers list
2. If exists → key is valid → proceed
3. If not exists → key is invalid → re-register
```

#### REQ-7: Auto-Heal Invalid Keys

**WHEN** a configured API key is not registered with CrowdSec, **THE SYSTEM SHALL** delete the bouncer and re-register to get a valid key.

**Acceptance Criteria**:

- [ ] Stale bouncer entries are deleted before re-registration
- [ ] New key is saved to file
- [ ] Log warning about invalid key being replaced
- [ ] Caddy config regenerated with new key

#### REQ-8: Display Key in UI

**WHEN** viewing CrowdSec settings in the Security dashboard, **THE SYSTEM SHALL** display the current bouncer API key (masked with copy button).

**Acceptance Criteria**:

- [ ] Key displayed as `PNoO...GSs` (first 4, last 3 chars)
- [ ] Copy button copies full key to clipboard
- [ ] Toast notification confirms copy
- [ ] Key only shown when CrowdSec is enabled

**UI Mockup**:

```text
┌─────────────────────────────────────────────────────────────────┐
│ CrowdSec Integration                              [Enabled ✓]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Status: Running (PID: 12345)                                  │
│  LAPI: http://127.0.0.1:8085 (Healthy)                         │
│                                                                 │
│  Bouncer API Key                                               │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │ PNoO...GSs                                    [📋 Copy] │  │
│  └─────────────────────────────────────────────────────────┘  │
│  Key stored at: /app/data/crowdsec/bouncer_key                 │
│                                                                 │
│  [Configure CrowdSec →]  [View Decisions]  [Manage Bouncers]  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Nice to Have (P2)

#### REQ-9: Key Rotation

**WHEN** user clicks "Rotate Key" button in UI, **THE SYSTEM SHALL** delete the old bouncer, register a new one, and update configuration.

**Acceptance Criteria**:

- [ ] Confirmation dialog before rotation
- [ ] Old bouncer deleted from CrowdSec
- [ ] New bouncer registered
- [ ] New key saved to file
- [ ] Caddy config regenerated
- [ ] Log new key to container logs

#### REQ-10: External CrowdSec Support

**WHEN** `CHARON_SECURITY_CROWDSEC_MODE=remote` is set, **THE SYSTEM SHALL** skip auto-registration and require manual key configuration.

**Acceptance Criteria**:

- [ ] Auto-registration disabled when mode=remote
- [ ] Clear error message if no key provided for remote mode
- [ ] Documentation updated with remote setup instructions

---

## Technical Design

### Implementation Overview

| Phase | Component | Changes |
|-------|-----------|---------|
| 1 | Backend | Add `ensureBouncerRegistered()` to CrowdSec Start handler |
| 2 | Backend | Update `getCrowdSecAPIKey()` with file fallback |
| 3 | Backend | Add bouncer validation and auto-heal logic |
| 4 | Backend | Add API endpoint to get bouncer key info |
| 5 | Frontend | Add bouncer key display to CrowdSec settings |
| 6 | Entrypoint | Create key persistence directory on startup |

### Phase 1: Bouncer Auto-Registration

**File**: `backend/internal/api/handlers/crowdsec_handler.go`

**Changes to `Start()` method** (after LAPI readiness check, ~line 290):

```go
// After confirming LAPI is ready, ensure bouncer is registered
if lapiReady {
    // Register bouncer if needed (idempotent)
    apiKey, regErr := h.ensureBouncerRegistration(ctx)
    if regErr != nil {
        logger.Log().WithError(regErr).Warn("Failed to register bouncer, CrowdSec may not enforce decisions")
    } else if apiKey != "" {
        // Log the key for user reference
        h.logBouncerKeyBanner(apiKey)

        // Regenerate Caddy config with new API key
        if h.CaddyManager != nil {
            if err := h.CaddyManager.ReloadConfig(ctx); err != nil {
                logger.Log().WithError(err).Warn("Failed to reload Caddy config with new bouncer key")
            }
        }
    }
}
```

**New method `ensureBouncerRegistration()`**:

```go
const (
    bouncerKeyFile = "/app/data/crowdsec/bouncer_key"
    bouncerName    = "caddy-bouncer"
)

// ensureBouncerRegistration checks if bouncer is registered and registers if needed.
// Returns the API key if newly generated (empty if already set via env var).
func (h *CrowdsecHandler) ensureBouncerRegistration(ctx context.Context) (string, error) {
    // Priority 1: Check environment variables
    envKey := getBouncerAPIKeyFromEnv()
    if envKey != "" {
        if h.validateBouncerKey(ctx) {
            logger.Log().Info("Using CrowdSec API key from environment variable")
            return "", nil // Key valid, nothing new to report
        }
        logger.Log().Warn("Env-provided CrowdSec API key is invalid or bouncer not registered, will re-register")
    }

    // Priority 2: Check persistent key file
    fileKey := readKeyFromFile(bouncerKeyFile)
    if fileKey != "" {
        if h.validateBouncerKey(ctx) {
            logger.Log().WithField("file", bouncerKeyFile).Info("Using CrowdSec API key from file")
            return "", nil // Key valid
        }
        logger.Log().WithField("file", bouncerKeyFile).Warn("File API key is invalid, will re-register")
    }

    // No valid key found - register new bouncer
    return h.registerAndSaveBouncer(ctx)
}

// validateBouncerKey checks if 'caddy-bouncer' is registered with CrowdSec.
func (h *CrowdsecHandler) validateBouncerKey(ctx context.Context) bool {
    checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    output, err := h.CmdExec.Execute(checkCtx, "cscli", "bouncers", "list", "-o", "json")
    if err != nil {
        logger.Log().WithError(err).Debug("Failed to list bouncers")
        return false
    }

    var bouncers []struct {
        Name string `json:"name"`
    }
    if err := json.Unmarshal(output, &bouncers); err != nil {
        return false
    }

    for _, b := range bouncers {
        if b.Name == bouncerName {
            return true
        }
    }
    return false
}

// registerAndSaveBouncer registers a new bouncer and saves the key to file.
func (h *CrowdsecHandler) registerAndSaveBouncer(ctx context.Context) (string, error) {
    // Delete existing bouncer if present (stale registration)
    deleteCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    h.CmdExec.Execute(deleteCtx, "cscli", "bouncers", "delete", bouncerName)
    cancel()

    // Register new bouncer
    regCtx, regCancel := context.WithTimeout(ctx, 10*time.Second)
    defer regCancel()

    output, err := h.CmdExec.Execute(regCtx, "cscli", "bouncers", "add", bouncerName, "-o", "raw")
    if err != nil {
        return "", fmt.Errorf("bouncer registration failed: %w: %s", err, string(output))
    }

    apiKey := strings.TrimSpace(string(output))
    if apiKey == "" {
        return "", fmt.Errorf("bouncer registration returned empty API key")
    }

    // Save key to persistent file
    if err := saveKeyToFile(bouncerKeyFile, apiKey); err != nil {
        logger.Log().WithError(err).Warn("Failed to save bouncer key to file")
        // Continue - key is still valid for this session
    }

    return apiKey, nil
}

// logBouncerKeyBanner logs the bouncer key with a formatted banner.
func (h *CrowdsecHandler) logBouncerKeyBanner(apiKey string) {
    banner := `
════════════════════════════════════════════════════════════════════
🔐 CrowdSec Bouncer Registered Successfully
────────────────────────────────────────────────────────────────────
Bouncer Name: %s
API Key:      %s
Saved To:     %s
────────────────────────────────────────────────────────────────────
💡 TIP: If connecting to an EXTERNAL CrowdSec instance, copy this
   key to your docker-compose.yml as CHARON_SECURITY_CROWDSEC_API_KEY
════════════════════════════════════════════════════════════════════`
    logger.Log().Infof(banner, bouncerName, apiKey, bouncerKeyFile)
}

// Helper functions
func getBouncerAPIKeyFromEnv() string {
    envVars := []string{
        "CROWDSEC_API_KEY",
        "CROWDSEC_BOUNCER_API_KEY",
        "CERBERUS_SECURITY_CROWDSEC_API_KEY",
        "CHARON_SECURITY_CROWDSEC_API_KEY",
        "CPM_SECURITY_CROWDSEC_API_KEY",
    }
    for _, key := range envVars {
        if val := os.Getenv(key); val != "" {
            return val
        }
    }
    return ""
}

func readKeyFromFile(path string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(data))
}

func saveKeyToFile(path string, key string) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0750); err != nil {
        return fmt.Errorf("create directory: %w", err)
    }

    if err := os.WriteFile(path, []byte(key+"\n"), 0600); err != nil {
        return fmt.Errorf("write key file: %w", err)
    }

    return nil
}
```

### Phase 2: Update getCrowdSecAPIKey() with File Fallback

**File**: `backend/internal/caddy/config.go`

**Replace current `getCrowdSecAPIKey()` implementation** (~line 1129):

```go
// getCrowdSecAPIKey retrieves the CrowdSec bouncer API key.
// Priority: environment variables > persistent key file
func getCrowdSecAPIKey() string {
    // Priority 1: Check environment variables
    envVars := []string{
        "CROWDSEC_API_KEY",
        "CROWDSEC_BOUNCER_API_KEY",
        "CERBERUS_SECURITY_CROWDSEC_API_KEY",
        "CHARON_SECURITY_CROWDSEC_API_KEY",
        "CPM_SECURITY_CROWDSEC_API_KEY",
    }

    for _, key := range envVars {
        if val := os.Getenv(key); val != "" {
            return val
        }
    }

    // Priority 2: Check persistent key file
    const bouncerKeyFile = "/app/data/crowdsec/bouncer_key"
    if data, err := os.ReadFile(bouncerKeyFile); err == nil {
        key := strings.TrimSpace(string(data))
        if key != "" {
            return key
        }
    }

    return ""
}
```

### Phase 3: Add Bouncer Key Info API Endpoint

**File**: `backend/internal/api/handlers/crowdsec_handler.go`

**New endpoint**: `GET /api/v1/admin/crowdsec/bouncer`

```go
// BouncerInfo represents the bouncer key information for UI display.
type BouncerInfo struct {
    Name       string `json:"name"`
    KeyPreview string `json:"key_preview"` // First 4 + last 3 chars
    KeySource  string `json:"key_source"`  // "env_var" | "file" | "none"
    FilePath   string `json:"file_path"`
    Registered bool   `json:"registered"`
}

// GetBouncerInfo returns information about the current bouncer key.
// GET /api/v1/admin/crowdsec/bouncer
func (h *CrowdsecHandler) GetBouncerInfo(c *gin.Context) {
    ctx := c.Request.Context()

    info := BouncerInfo{
        Name:     bouncerName,
        FilePath: bouncerKeyFile,
    }

    // Determine key source
    envKey := getBouncerAPIKeyFromEnv()
    fileKey := readKeyFromFile(bouncerKeyFile)

    var fullKey string
    if envKey != "" {
        info.KeySource = "env_var"
        fullKey = envKey
    } else if fileKey != "" {
        info.KeySource = "file"
        fullKey = fileKey
    } else {
        info.KeySource = "none"
    }

    // Generate preview
    if fullKey != "" && len(fullKey) > 7 {
        info.KeyPreview = fullKey[:4] + "..." + fullKey[len(fullKey)-3:]
    } else if fullKey != "" {
        info.KeyPreview = "***"
    }

    // Check if bouncer is registered
    info.Registered = h.validateBouncerKey(ctx)

    c.JSON(http.StatusOK, info)
}

// GetBouncerKey returns the full bouncer key (for copy to clipboard).
// GET /api/v1/admin/crowdsec/bouncer/key
func (h *CrowdsecHandler) GetBouncerKey(c *gin.Context) {
    envKey := getBouncerAPIKeyFromEnv()
    if envKey != "" {
        c.JSON(http.StatusOK, gin.H{"key": envKey, "source": "env_var"})
        return
    }

    fileKey := readKeyFromFile(bouncerKeyFile)
    if fileKey != "" {
        c.JSON(http.StatusOK, gin.H{"key": fileKey, "source": "file"})
        return
    }

    c.JSON(http.StatusNotFound, gin.H{"error": "No bouncer key configured"})
}
```

**Add routes**: `backend/internal/api/routes/routes.go`

```go
// Add to CrowdSec admin routes section
crowdsec.GET("/bouncer", crowdsecHandler.GetBouncerInfo)
crowdsec.GET("/bouncer/key", crowdsecHandler.GetBouncerKey)
```

### Phase 4: Frontend - Display Bouncer Key in UI

**File**: `frontend/src/pages/Security.tsx` (or create new component)

**New component**: `frontend/src/components/CrowdSecBouncerKeyDisplay.tsx`

```tsx
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Copy, Check, Key, AlertCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';

interface BouncerInfo {
  name: string;
  key_preview: string;
  key_source: 'env_var' | 'file' | 'none';
  file_path: string;
  registered: boolean;
}

async function fetchBouncerInfo(): Promise<BouncerInfo> {
  const res = await fetch('/api/v1/admin/crowdsec/bouncer');
  if (!res.ok) throw new Error('Failed to fetch bouncer info');
  return res.json();
}

async function fetchBouncerKey(): Promise<string> {
  const res = await fetch('/api/v1/admin/crowdsec/bouncer/key');
  if (!res.ok) throw new Error('No key available');
  const data = await res.json();
  return data.key;
}

export function CrowdSecBouncerKeyDisplay() {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const { data: info, isLoading } = useQuery({
    queryKey: ['crowdsec-bouncer-info'],
    queryFn: fetchBouncerInfo,
    refetchInterval: 30000, // Refresh every 30s
  });

  const handleCopyKey = async () => {
    try {
      const key = await fetchBouncerKey();
      await navigator.clipboard.writeText(key);
      setCopied(true);
      toast.success(t('security.crowdsec.keyCopied'));
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error(t('security.crowdsec.copyFailed'));
    }
  };

  if (isLoading || !info) {
    return null;
  }

  if (info.key_source === 'none') {
    return (
      <Card className="border-yellow-200 bg-yellow-50">
        <CardContent className="flex items-center gap-2 py-3">
          <AlertCircle className="h-4 w-4 text-yellow-600" />
          <span className="text-sm text-yellow-800">
            {t('security.crowdsec.noKeyConfigured')}
          </span>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-base">
          <Key className="h-4 w-4" />
          {t('security.crowdsec.bouncerApiKey')}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between">
          <code className="rounded bg-muted px-2 py-1 font-mono text-sm">
            {info.key_preview}
          </code>
          <Button
            variant="outline"
            size="sm"
            onClick={handleCopyKey}
            disabled={copied}
          >
            {copied ? (
              <>
                <Check className="mr-1 h-3 w-3" />
                {t('common.copied')}
              </>
            ) : (
              <>
                <Copy className="mr-1 h-3 w-3" />
                {t('common.copy')}
              </>
            )}
          </Button>
        </div>

        <div className="flex flex-wrap gap-2">
          <Badge variant={info.registered ? 'default' : 'destructive'}>
            {info.registered
              ? t('security.crowdsec.registered')
              : t('security.crowdsec.notRegistered')}
          </Badge>
          <Badge variant="outline">
            {info.key_source === 'env_var'
              ? t('security.crowdsec.sourceEnvVar')
              : t('security.crowdsec.sourceFile')}
          </Badge>
        </div>

        <p className="text-xs text-muted-foreground">
          {t('security.crowdsec.keyStoredAt')}: <code>{info.file_path}</code>
        </p>
      </CardContent>
    </Card>
  );
}
```

**Add translations**: `frontend/src/i18n/en.json`

```json
{
  "security": {
    "crowdsec": {
      "bouncerApiKey": "Bouncer API Key",
      "keyCopied": "API key copied to clipboard",
      "copyFailed": "Failed to copy API key",
      "noKeyConfigured": "No bouncer key configured. Enable CrowdSec to auto-register.",
      "registered": "Registered",
      "notRegistered": "Not Registered",
      "sourceEnvVar": "From environment variable",
      "sourceFile": "From file",
      "keyStoredAt": "Key stored at"
    }
  }
}
```

### Phase 5: Update Docker Entrypoint

**File**: `.docker/docker-entrypoint.sh`

**Add key directory creation** (after line ~45, in the CrowdSec initialization section):

```bash
# ============================================================================
# CrowdSec Key Persistence Directory
# ============================================================================
# Create the persistent directory for bouncer key storage.
# This directory is inside /app/data which is volume-mounted.

CS_KEY_DIR="/app/data/crowdsec"
mkdir -p "$CS_KEY_DIR" 2>/dev/null || echo "Warning: Cannot create $CS_KEY_DIR"

# Fix ownership for key directory
if is_root; then
    chown charon:charon "$CS_KEY_DIR" 2>/dev/null || true
fi

# Create symlink for backwards compatibility with register_bouncer.sh
BOUNCER_DIR="/etc/crowdsec/bouncers"
if [ ! -d "$BOUNCER_DIR" ]; then
    mkdir -p "$BOUNCER_DIR" 2>/dev/null || true
fi

# Log key location for user reference
echo "CrowdSec bouncer key will be stored at: $CS_KEY_DIR/bouncer_key"
```

---

## Test Scenarios

### Playwright E2E Tests

**File**: `tests/crowdsec/bouncer-auto-registration.spec.ts`

```typescript
import { test, expect } from '@playwright/test';
import { loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('CrowdSec Bouncer Auto-Registration', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await page.goto('/security');
    await waitForLoadingComplete(page);
  });

  test('Scenario 1: Fresh install - auto-registers bouncer on CrowdSec enable', async ({ page }) => {
    await test.step('Enable CrowdSec', async () => {
      const crowdsecToggle = page.getByRole('switch', { name: /crowdsec/i });
      await crowdsecToggle.click();

      // Wait for CrowdSec to start (can take up to 30s)
      await expect(page.getByText(/running/i)).toBeVisible({ timeout: 45000 });
    });

    await test.step('Verify bouncer key is displayed', async () => {
      // Should show bouncer key card after registration
      await expect(page.getByText(/bouncer api key/i)).toBeVisible();
      await expect(page.getByText(/registered/i)).toBeVisible();

      // Key preview should be visible
      const keyPreview = page.locator('code').filter({ hasText: /.../ });
      await expect(keyPreview).toBeVisible();
    });

    await test.step('Copy key to clipboard', async () => {
      const copyButton = page.getByRole('button', { name: /copy/i });
      await copyButton.click();

      // Verify toast notification
      await expect(page.getByText(/copied/i)).toBeVisible();
    });
  });

  test('Scenario 2: Invalid env var key - auto-heals by re-registering', async ({ page, request }) => {
    // This test requires setting an invalid env var - may need Docker restart
    // or API-based configuration change
    test.skip(true, 'Requires container restart with invalid env var');
  });

  test('Scenario 3: Key persists across CrowdSec restart', async ({ page }) => {
    await test.step('Enable CrowdSec and note key', async () => {
      const crowdsecToggle = page.getByRole('switch', { name: /crowdsec/i });
      if (!(await crowdsecToggle.isChecked())) {
        await crowdsecToggle.click();
        await expect(page.getByText(/running/i)).toBeVisible({ timeout: 45000 });
      }
    });

    const keyPreview = await page.locator('code').filter({ hasText: /.../ }).textContent();

    await test.step('Stop CrowdSec', async () => {
      const crowdsecToggle = page.getByRole('switch', { name: /crowdsec/i });
      await crowdsecToggle.click();
      await expect(page.getByText(/stopped/i)).toBeVisible({ timeout: 10000 });
    });

    await test.step('Re-enable CrowdSec', async () => {
      const crowdsecToggle = page.getByRole('switch', { name: /crowdsec/i });
      await crowdsecToggle.click();
      await expect(page.getByText(/running/i)).toBeVisible({ timeout: 45000 });
    });

    await test.step('Verify same key is used', async () => {
      const newKeyPreview = await page.locator('code').filter({ hasText: /.../ }).textContent();
      expect(newKeyPreview).toBe(keyPreview);
    });
  });
});
```

### Go Unit Tests

**File**: `backend/internal/api/handlers/crowdsec_handler_bouncer_test.go`

```go
package handlers

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEnsureBouncerRegistration_EnvVarPriority(t *testing.T) {
    // Set env var
    t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "test-key-from-env")

    key := getBouncerAPIKeyFromEnv()
    assert.Equal(t, "test-key-from-env", key)
}

func TestEnsureBouncerRegistration_FileFallback(t *testing.T) {
    // Create temp directory
    tmpDir := t.TempDir()
    keyFile := filepath.Join(tmpDir, "bouncer_key")

    // Write key to file
    err := os.WriteFile(keyFile, []byte("test-key-from-file\n"), 0600)
    require.NoError(t, err)

    key := readKeyFromFile(keyFile)
    assert.Equal(t, "test-key-from-file", key)
}

func TestSaveKeyToFile(t *testing.T) {
    tmpDir := t.TempDir()
    keyFile := filepath.Join(tmpDir, "subdir", "bouncer_key")

    err := saveKeyToFile(keyFile, "new-api-key")
    require.NoError(t, err)

    // Verify file exists with correct permissions
    info, err := os.Stat(keyFile)
    require.NoError(t, err)
    assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

    // Verify content
    content, err := os.ReadFile(keyFile)
    require.NoError(t, err)
    assert.Equal(t, "new-api-key\n", string(content))
}

func TestGetCrowdSecAPIKey_PriorityOrder(t *testing.T) {
    tmpDir := t.TempDir()

    // Create key file
    keyFile := filepath.Join(tmpDir, "bouncer_key")
    os.WriteFile(keyFile, []byte("file-key"), 0600)

    // Test 1: Env var takes priority
    t.Run("env_var_priority", func(t *testing.T) {
        t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "env-key")
        key := getCrowdSecAPIKey()
        assert.Equal(t, "env-key", key)
    })

    // Test 2: File fallback when no env var
    t.Run("file_fallback", func(t *testing.T) {
        // Clear all env vars
        os.Unsetenv("CROWDSEC_API_KEY")
        os.Unsetenv("CHARON_SECURITY_CROWDSEC_API_KEY")
        // Note: Need to mock the file path for this test
    })
}
```

---

## Upgrade Path

### Existing Installations

| Scenario | Current State | After Upgrade |
|----------|---------------|---------------|
| No env var, no key file | CrowdSec fails silently | Auto-registers on enable |
| Invalid env var | "access forbidden" errors | Auto-heals, re-registers |
| Valid env var | Works | No change |
| Key file exists (from manual script) | Never read | Now used as fallback |

### Migration Steps

1. **No action required by users** - upgrade happens automatically
2. **Existing env var keys** continue to work (priority is preserved)
3. **First CrowdSec enable after upgrade** triggers auto-registration if needed
4. **Container logs contain the new key** for user reference

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing valid env var keys | Low | High | Env var always takes priority |
| `cscli` not available | Low | Medium | Check exists before calling |
| Key file permission issues | Low | Medium | Use 600 mode, catch errors |
| Race condition on startup | Low | Low | Registration happens after LAPI ready |
| External CrowdSec users confused | Medium | Low | Log message explains external setup |

---

## Implementation Checklist

### Phase 1: Backend Core (Estimated: 3-4 hours)

- [ ] Add `ensureBouncerRegistration()` method to `crowdsec_handler.go`
- [ ] Add `validateBouncerKey()` method
- [ ] Add `registerAndSaveBouncer()` method
- [ ] Add `logBouncerKeyBanner()` method
- [ ] Integrate into `Start()` method
- [ ] Add helper functions (`getBouncerAPIKeyFromEnv`, `readKeyFromFile`, `saveKeyToFile`)
- [ ] Write unit tests for new methods

### Phase 2: Config Loading (Estimated: 1 hour)

- [ ] Update `getCrowdSecAPIKey()` in `caddy/config.go` with file fallback
- [ ] Add unit tests for priority order
- [ ] Verify Caddy config regeneration uses new key

### Phase 3: API Endpoints (Estimated: 1 hour)

- [ ] Add `GET /api/v1/admin/crowdsec/bouncer` endpoint
- [ ] Add `GET /api/v1/admin/crowdsec/bouncer/key` endpoint
- [ ] Register routes
- [ ] Add API tests

### Phase 4: Frontend (Estimated: 2-3 hours)

- [ ] Create `CrowdSecBouncerKeyDisplay` component
- [ ] Add to Security page (CrowdSec section)
- [ ] Add translations (en, other locales as needed)
- [ ] Add copy-to-clipboard functionality
- [ ] Write component tests

### Phase 5: Docker Entrypoint (Estimated: 30 min)

- [ ] Add key directory creation to `docker-entrypoint.sh`
- [ ] Add symlink for backwards compatibility
- [ ] Test container startup

### Phase 6: Testing & Documentation (Estimated: 2-3 hours)

- [ ] Write Playwright E2E tests
- [ ] Update `docs/guides/crowdsec-setup.md`
- [ ] Update `docs/configuration.md` with new key location
- [ ] Update `CHANGELOG.md`
- [ ] Manual testing of all scenarios

---

## Files to Modify

| File | Changes |
|------|---------|
| `backend/internal/api/handlers/crowdsec_handler.go` | Add registration logic, new endpoints |
| `backend/internal/caddy/config.go` | Update `getCrowdSecAPIKey()` |
| `backend/internal/api/routes/routes.go` | Add bouncer routes |
| `.docker/docker-entrypoint.sh` | Add key directory creation |
| `frontend/src/components/CrowdSecBouncerKeyDisplay.tsx` | New component |
| `frontend/src/pages/Security.tsx` | Import and use new component |
| `frontend/src/i18n/en.json` | Add translations |
| `docs/guides/crowdsec-setup.md` | Update documentation |

---

## References

- [CrowdSec Bouncer Documentation](https://doc.crowdsec.net/docs/bouncers/intro)
- [Caddy CrowdSec Plugin](https://github.com/hslatman/caddy-crowdsec-bouncer)
- [Existing register_bouncer.sh](../../configs/crowdsec/register_bouncer.sh)
- [Related Plan: crowdsec_lapi_auth_fix.md](./crowdsec_lapi_auth_fix.md)
- [docker-entrypoint.sh](../../.docker/docker-entrypoint.sh)
- [CrowdSec Handler](../../backend/internal/api/handlers/crowdsec_handler.go)

---

**Last Updated**: 2026-02-03
**Owner**: TBD
**Reviewers**: TBD
