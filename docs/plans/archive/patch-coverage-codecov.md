# Patch Coverage Only (Codecov)

This plan is strictly about adding deterministic tests (and only micro-hooks if absolutely required) to push patch coverage ≥85%.

## What’s failing

Codecov **patch** coverage is failing at **82.64%**. The failure is specifically driven by missing coverage on newly-changed lines across **10 flagged backend files** (listed below).

### 10 flagged backend files (scope boundary)

1. `backend/internal/caddy/config.go`
2. `backend/internal/services/dns_provider_service.go`
3. `backend/internal/caddy/manager.go`
4. `backend/internal/utils/url_testing.go`
5. `backend/internal/network/safeclient.go`
6. `backend/internal/api/routes/routes.go`
7. `backend/internal/services/notification_service.go`
8. `backend/internal/crypto/encryption.go`
9. `backend/internal/api/handlers/settings_handler.go`
10. `backend/internal/security/url_validator.go`

----

## Phase 0 — Extract missing patch line ranges (required, exact steps)

Goal: for each of the 10 files, record the **exact line ranges** Codecov considers “missing” on the patch, then map each range to a specific branch and a specific new test.

### A) From GitHub PR Codecov view (fastest, authoritative)

1. Open the PR in GitHub.
2. Find the Codecov status check / comment, then open the **Codecov “Files changed”** (or “View report”) link.
3. In Codecov, switch to **Patch** (not Project).
4. Filter to the 10 backend files listed above.
5. For each file:
   - Click the file.
   - Identify **red** (uncovered) and **yellow** (partial) lines.
   - Note the **exact line numbers/ranges** as shown in Codecov.
6. Paste those ranges into the table template in “Phase 0 Output”.

### B) From local `go tool cover -html` (best for understanding branches)

1. Run the VS Code task `shell: Test: Backend with Coverage`.
2. Generate an HTML view:
   - `cd backend`
   - `go tool cover -html=coverage.txt -o ../test-results/backend_cover.html`
3. Open `test-results/backend_cover.html` in a browser.
4. For each of the 10 files:
   - Use in-page search for the filename (or navigate via the coverage page UI).
   - Identify red/uncovered spans and record the **exact line ranges**.
5. Cross-check that those spans overlap with the PR diff (next section) so you’re targeting **patch lines**, not legacy debt.

### C) From `git diff main...HEAD` (ground truth for “patch” lines)

1. List hunks for a file:
   - `git diff -U0 main...HEAD -- backend/internal/caddy/config.go`
2. For each hunk header like:
   - `@@ -OLDSTART,OLDCOUNT +NEWSTART,NEWCOUNT @@`
3. Convert it into a line range on the **new** file:
   - If `NEWCOUNT` is 0, that hunk adds no new lines.
   - Otherwise the patch line range is:
     - `NEWSTART` through `NEWSTART + NEWCOUNT - 1`
4. Repeat for each of the 10 files.

### Phase 0 Output: table to paste missing line ranges into

Paste and fill this as you extract line ranges (Codecov is the source of truth):

| File | Missing patch line ranges (Codecov) | Partial patch line ranges (Codecov) | Notes (branch / error path) |
|------|-------------------------------------|-------------------------------------|-----------------------------|
| `backend/internal/caddy/config.go` | TBD | TBD | |
| `backend/internal/services/dns_provider_service.go` | TBD | TBD | |
| `backend/internal/caddy/manager.go` | TBD | TBD | |
| `backend/internal/utils/url_testing.go` | TBD | TBD | |
| `backend/internal/network/safeclient.go` | TBD | TBD | |
| `backend/internal/api/routes/routes.go` | TBD | TBD | |
| `backend/internal/services/notification_service.go` | TBD | TBD | |
| `backend/internal/crypto/encryption.go` | TBD | TBD | |
| `backend/internal/api/handlers/settings_handler.go` | TBD | TBD | |
| `backend/internal/security/url_validator.go` | TBD | TBD | |

----

## File-by-file plan (tests only, implementation-ready)

### 1) `backend/internal/caddy/config.go`

**Targeted branches (functions/paths in this file)**

- `GenerateConfig`:
  - wildcard vs non-wildcard split (`hasWildcard(cleanDomains)`)
  - DNS-provider policy creation vs HTTP-challenge policy creation
  - `sslProvider` switch: `letsencrypt` vs `zerossl` vs default/both
  - `acmeStaging` CA override branch
  - “DNS provider config missing → skip policy” (`dnsProviderMap` lookup miss)
  - IP filtering for ACME issuers (`net.ParseIP` branch)
- `getCrowdSecAPIKey`: env-var priority / fallback branches
- `hasWildcard`: true/false detection

**Tests to add (exact paths + exact test function names)**

- Add file: `backend/internal/caddy/config_patch_coverage_test.go`
  - `TestGenerateConfig_DNSChallenge_LetsEncrypt_StagingCAAndPropagationTimeout`
  - `TestGenerateConfig_DNSChallenge_ZeroSSL_IssuerShape`
  - `TestGenerateConfig_DNSChallenge_SkipsPolicyWhenProviderConfigMissing`
  - `TestGenerateConfig_HTTPChallenge_ExcludesIPDomains`
  - `TestGetCrowdSecAPIKey_EnvPriority`
  - `TestHasWildcard_TrueFalse`

**Determinism**

- Use pure in-memory inputs; no Docker/Caddy process calls.
- Avoid comparing raw JSON strings; unmarshal and assert keys/values.
- When asserting lists (subjects), use set semantics or sort before compare.

### 2) `backend/internal/services/dns_provider_service.go`

**Targeted branches (functions/paths in this file)**

- `(*dnsProviderService).Create`:
  - invalid provider type
  - credential validation error
  - defaults (`PropagationTimeout == 0`, `PollingInterval == 0`)
  - `IsDefault` true → unset previous defaults branch
- `(*dnsProviderService).Update`:
  - credentials nil/empty vs non-empty
  - `IsDefault` toggle branches
- `(*dnsProviderService).Delete`: `RowsAffected == 0` not found
- `(*dnsProviderService).Test`: decryption failure returns a structured `TestResult` (no error)
- `(*dnsProviderService).GetDecryptedCredentials`: decrypt error vs invalid JSON error

**Tests to add (exact paths + exact test function names)**

- Update file: `backend/internal/services/dns_provider_service_test.go`
  - `TestDNSProviderServiceCreate_DefaultsAndUnsetsExistingDefault`
  - `TestDNSProviderServiceUpdate_DoesNotOverwriteCredentialsWhenEmpty`
  - `TestDNSProviderServiceDelete_NotFound`
  - `TestDNSProviderServiceTest_DecryptionError_ReturnsResultNotErr`
  - `TestDNSProviderServiceGetDecryptedCredentials_InvalidJSON_ReturnsErrDecryptionFailed`

**Determinism**

- Use in-memory sqlite via existing test helpers.
- Avoid map ordering assumptions in comparisons.

### 3) `backend/internal/caddy/manager.go`

**Targeted branches (functions/paths in this file)**

- `(*Manager).ApplyConfig`:
  - DNS provider load present vs empty (`len(dnsProviders) > 0`)
  - encryption key discovery:
    - `CHARON_ENCRYPTION_KEY` set
    - fallback keys (`ENCRYPTION_KEY`, `CERBERUS_ENCRYPTION_KEY`)
    - no key → warning branch
  - decrypt/parse branches per provider:
    - `CredentialsEncrypted == ""` skip
    - decrypt error skip
    - JSON unmarshal error skip

**Tests to add (exact paths + exact test function names)**

- Add file: `backend/internal/caddy/manager_patch_coverage_test.go`
  - `TestManagerApplyConfig_DNSProviders_NoKey_SkipsDecryption`
  - `TestManagerApplyConfig_DNSProviders_UsesFallbackEnvKeys`
  - `TestManagerApplyConfig_DNSProviders_SkipsDecryptOrJSONFailures`

**Determinism**

- Do not hit real filesystem/Caddy Admin API; override `generateConfigFunc` / `validateConfigFunc` to capture inputs and short-circuit.
- Do not use `t.Parallel()` unless you fully isolate and restore package-level hooks.
- Use `t.Setenv` for env changes.

### 4) `backend/internal/utils/url_testing.go`

**Targeted branches (functions/paths in this file)**

- `TestURLConnectivity`:
  - scheme validation (`http/https` only)
  - production path (no transport): validation error mapping
  - test path (custom transport): “skip DNS/IP validation” branch
  - redirect handling: `validateRedirectTarget` (localhost allowlist vs validation failure)
- `ssrfSafeDialer`: address parse error, DNS resolution error, “any private IP blocks” branch

**Tests to add (exact paths + exact test function names)**

- Update file: `backend/internal/utils/url_testing_test.go`
  - `TestValidateRedirectTarget_AllowsLocalhost`
  - `TestValidateRedirectTarget_BlocksInvalidExternalRedirect`
  - `TestURLConnectivity_TestPath_ReconstructsURLAndSkipsDNSValidation`

**Determinism**

- Use `httptest.Server` + injected `http.RoundTripper` to avoid real DNS/network.
- Avoid timing flakiness: assert “latency > 0” only when a local server is used.

### 5) `backend/internal/network/safeclient.go`

**Targeted branches (functions/paths in this file)**

- `IsPrivateIP`: fast-path checks and CIDR-block scan
- `safeDialer`:
  - bad `host:port` parse
  - localhost allow/deny branch (`AllowLocalhost`)
  - DNS lookup error / no IPs
  - “any private IP blocks” validation branch
- `validateRedirectTarget`: missing hostname, private IP block

**Tests to add (exact paths + exact test function names)**

- Update file: `backend/internal/network/safeclient_test.go`
  - `TestValidateRedirectTarget_MissingHostname_ReturnsErr`
  - `TestValidateRedirectTarget_BlocksPrivateIPRedirect`
  - `TestNewSafeHTTPClient_WithMaxRedirects_EnforcesRedirectLimit`

**Determinism**

- Prefer redirect tests built with `httptest.Server`.
- Keep tests self-contained by controlling `Location` headers.

### 6) `backend/internal/api/routes/routes.go`

**Targeted branches (functions/paths in this file)**

- `Register`:
  - DNS providers are conditionally registered only when `cfg.EncryptionKey != ""`.
  - Branch when `crypto.NewEncryptionService(cfg.EncryptionKey)` fails (routes should remain unavailable).
  - Branch when encryption service initializes successfully (routes should exist).

**Tests to add (exact paths + exact test function names)**

- Update file: `backend/internal/api/routes/routes_test.go`
  - `TestRegister_DNSProviders_NotRegisteredWhenEncryptionKeyMissing`
  - `TestRegister_DNSProviders_NotRegisteredWhenEncryptionKeyInvalid`
  - `TestRegister_DNSProviders_RegisteredWhenEncryptionKeyValid`

**Determinism**

- Validate route presence by inspecting `router.Routes()` and asserting `"/dns-providers"` paths exist or do not exist.
- Use a known-good base64 32-byte key literal for “valid” cases.

### 7) `backend/internal/services/notification_service.go`

**Targeted branches (functions/paths in this file)**

- `normalizeURL`: Discord webhook URL normalization vs passthrough
- `SendExternal`:
  - event-type preference filtering branches
  - http/https destination validation using `security.ValidateExternalURL` (skip invalid destinations)
  - JSON-template path vs shoutrrr path
- `sendJSONPayload`:
  - template selection (`minimal`/`detailed`/`custom`/default)
  - template size limit
  - URL validation failure branch
  - template parse/execute error branches
  - JSON unmarshal error
  - service-specific validation branches (discord/slack/gotify)

**Tests to add (exact paths + exact test function names)**

- Update file: `backend/internal/services/notification_service_json_test.go`
  - `TestNormalizeURL_DiscordWebhook_ConvertsToDiscordScheme`
  - `TestSendExternal_SkipsInvalidHTTPDestination`
  - `TestSendJSONPayload_InvalidTemplate_ReturnsErr`

**Determinism**

- Avoid real external calls: use `httptest.Server` and validation-only failures.
- For `SendExternal` (async goroutine), use an `atomic` counter + short `time.After` guard to assert “no send happened”.

### 8) `backend/internal/crypto/encryption.go`

**Targeted branches (functions/paths in this file)**

- `NewEncryptionService`: invalid base64, wrong key length
- `(*EncryptionService).Encrypt`: nonce generation
- `(*EncryptionService).Decrypt`: invalid base64, ciphertext too short, auth/tag failure (`gcm.Open`)

**Tests to add (exact paths + exact test function names)**

- Update file: `backend/internal/crypto/encryption_test.go`
  - `TestDecrypt_InvalidCiphertext` (extend table cases if patch adds new error branches)
  - `TestDecrypt_TamperedCiphertext` (ensure it hits the new/changed patch lines)

**Determinism**

- Avoid any network/file IO.
- Do not rely on ciphertext equality; only assert round-trips and error substrings.

### 9) `backend/internal/api/handlers/settings_handler.go`

**Targeted branches (functions/handlers in this file)**

- `ValidatePublicURL`:
  - admin-only gate (403)
  - JSON bind error (400)
  - invalid URL format (400)
  - success response includes optional warning field
- `TestPublicURL`:
  - admin-only gate (403)
  - JSON bind error (400)
  - format validation error (400)
  - SSRF validation failure returns 200 with `reachable=false`
  - connectivity error returns 200 with `reachable=false`
  - success path returns 200 with `reachable` and `latency`

**Tests to add (exact paths + exact test function names)**

- Update file: `backend/internal/api/handlers/settings_handler_test.go`
  - `TestSettingsHandler_ValidatePublicURL_NonAdmin_Returns403`
  - `TestSettingsHandler_TestPublicURL_BindError_Returns400`
  - `TestSettingsHandler_TestPublicURL_SSRFValidationFailure_Returns200ReachableFalse`

**Determinism**

- Use gin test mode + httptest recorder; don’t hit real DNS.
- For `TestPublicURL`, choose inputs that fail at validation (e.g., `http://10.0.0.1`) so the handler exits before any real network.

### 10) `backend/internal/security/url_validator.go`

**Targeted branches (functions/paths in this file)**

- `ValidateExternalURL`:
  - scheme enforcement (https-only unless `WithAllowHTTP`)
  - credentials-in-URL rejection
  - hostname length guard and suspicious pattern rejection
  - port parsing and privileged-port blocking (non-standard ports <1024)
  - `WithAllowLocalhost` early-return path for localhost/127.0.0.1/::1
  - DNS resolution failure path
  - private IP block path (including IPv4-mapped IPv6 detection)

**Tests to add (exact paths + exact test function names)**

- Update file: `backend/internal/security/url_validator_test.go`
  - `TestValidateExternalURL_HostnameTooLong_ReturnsErr`
  - `TestValidateExternalURL_SuspiciousHostnamePattern_ReturnsErr`
  - `TestValidateExternalURL_PrivilegedPortBlocked_ReturnsErr`
  - `TestValidateExternalURL_URLWithCredentials_ReturnsErr`

**Determinism**

- Prefer validation-only failures (length/pattern/credentials/port) to avoid reliance on DNS.
- Avoid asserting exact DNS error strings; assert on stable substrings for validation errors.

----

## Patch Coverage Hit List (consolidated)

Fill the missing ranges from Phase 0, then implement only the tests listed here.

| File | Missing patch line ranges | Test(s) that hit it |
|------|---------------------------|---------------------|
| `backend/internal/caddy/config.go` | TBD: fill from Codecov | `TestGenerateConfig_DNSChallenge_LetsEncrypt_StagingCAAndPropagationTimeout`; `TestGenerateConfig_DNSChallenge_ZeroSSL_IssuerShape`; `TestGenerateConfig_DNSChallenge_SkipsPolicyWhenProviderConfigMissing`; `TestGenerateConfig_HTTPChallenge_ExcludesIPDomains`; `TestGetCrowdSecAPIKey_EnvPriority`; `TestHasWildcard_TrueFalse` |
| `backend/internal/services/dns_provider_service.go` | TBD: fill from Codecov | `TestDNSProviderServiceCreate_DefaultsAndUnsetsExistingDefault`; `TestDNSProviderServiceUpdate_DoesNotOverwriteCredentialsWhenEmpty`; `TestDNSProviderServiceDelete_NotFound`; `TestDNSProviderServiceTest_DecryptionError_ReturnsResultNotErr`; `TestDNSProviderServiceGetDecryptedCredentials_InvalidJSON_ReturnsErrDecryptionFailed` |
| `backend/internal/caddy/manager.go` | TBD: fill from Codecov | `TestManagerApplyConfig_DNSProviders_NoKey_SkipsDecryption`; `TestManagerApplyConfig_DNSProviders_UsesFallbackEnvKeys`; `TestManagerApplyConfig_DNSProviders_SkipsDecryptOrJSONFailures` |
| `backend/internal/utils/url_testing.go` | TBD: fill from Codecov | `TestValidateRedirectTarget_AllowsLocalhost`; `TestValidateRedirectTarget_BlocksInvalidExternalRedirect`; `TestURLConnectivity_TestPath_ReconstructsURLAndSkipsDNSValidation` |
| `backend/internal/network/safeclient.go` | TBD: fill from Codecov | `TestValidateRedirectTarget_MissingHostname_ReturnsErr`; `TestValidateRedirectTarget_BlocksPrivateIPRedirect`; `TestNewSafeHTTPClient_WithMaxRedirects_EnforcesRedirectLimit` |
| `backend/internal/api/routes/routes.go` | TBD: fill from Codecov | `TestRegister_DNSProviders_NotRegisteredWhenEncryptionKeyMissing`; `TestRegister_DNSProviders_NotRegisteredWhenEncryptionKeyInvalid`; `TestRegister_DNSProviders_RegisteredWhenEncryptionKeyValid` |
| `backend/internal/services/notification_service.go` | TBD: fill from Codecov | `TestNormalizeURL_DiscordWebhook_ConvertsToDiscordScheme`; `TestSendExternal_SkipsInvalidHTTPDestination`; `TestSendJSONPayload_InvalidTemplate_ReturnsErr` |
| `backend/internal/crypto/encryption.go` | TBD: fill from Codecov | `TestDecrypt_InvalidCiphertext`; `TestDecrypt_TamperedCiphertext` |
| `backend/internal/api/handlers/settings_handler.go` | TBD: fill from Codecov | `TestSettingsHandler_ValidatePublicURL_NonAdmin_Returns403`; `TestSettingsHandler_TestPublicURL_BindError_Returns400`; `TestSettingsHandler_TestPublicURL_SSRFValidationFailure_Returns200ReachableFalse` |
| `backend/internal/security/url_validator.go` | TBD: fill from Codecov | `TestValidateExternalURL_HostnameTooLong_ReturnsErr`; `TestValidateExternalURL_SuspiciousHostnamePattern_ReturnsErr`; `TestValidateExternalURL_PrivilegedPortBlocked_ReturnsErr`; `TestValidateExternalURL_URLWithCredentials_ReturnsErr` |

----

## Minimal refactors (only if required)

Do not refactor unless Phase 0 shows patch-missing lines are in branches that are effectively unreachable from tests.

- `backend/internal/services/dns_provider_service.go`: if `json.Marshal` failure branches in `Create`/`Update` are patch-missing and cannot be triggered via normal inputs, add a micro-hook (package-level `var marshalJSON = json.Marshal`) and restore it in tests.
- `backend/internal/caddy/manager.go`: if any error branches depend on OS/JSON behaviors, use the existing package-level hooks (e.g., `readFileFunc`, `writeFileFunc`, `jsonMarshalFunc`, `generateConfigFunc`) and **always restore** them with `defer`.

Notes:

- Avoid `t.Parallel()` in any test that touches package-level vars/hooks unless you fully isolate state.
- Any global override must be restored, even on failure paths.

----

## Validation checklist (run after tests are implemented)

- `shell: Test: Backend with Coverage`
- `shell: Lint: Pre-commit (All Files)`
- `shell: Security: CodeQL Go Scan (CI-Aligned) [~60s]`
- `shell: Security: CodeQL JS Scan (CI-Aligned) [~90s]`
- `shell: Security: Trivy Scan`
