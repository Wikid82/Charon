# Changelog

All notable changes to Charon will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Repository Structure Reorganization**: Cleaned up root directory for better navigation
  - Moved docker-compose files to `.docker/compose/`
  - Moved `docker-entrypoint.sh` to `.docker/`
  - Moved 16 implementation docs to `docs/implementation/`
  - Deleted test artifacts (`block_test.txt`, `caddy_*.json`, etc.)
  - Added `.github/instructions/structure.instructions.md` for ongoing structure enforcement

### Added

- **Bulk Apply Security Header Profiles**: Apply or remove security header profiles from multiple proxy hosts simultaneously via the Bulk Apply modal
- **Standard Proxy Headers**: Charon now adds X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, and
  X-Forwarded-Port headers to all proxy hosts by default. This enables proper client IP detection,
  HTTPS enforcement, and logging in backend applications.
  - New feature flag: `enable_standard_headers` (default: true for new hosts, false for existing)
  - UI: Checkbox in proxy host form with info banner explaining backward compatibility
  - Bulk operations: Toggle available in bulk apply modal for enabling/disabling across multiple hosts
  - Migration path: Existing hosts preserve old behavior (headers disabled) for backward compatibility
  - Note: X-Forwarded-For is handled natively by Caddy and not explicitly set by Charon

### Changed

- **Backend Applications**: Applications behind Charon proxies will now receive client IP and protocol
  information via standard headers when the feature is enabled

### Fixed

- Fixed 500 error when saving proxy hosts caused by invalid `trusted_proxies` structure in Caddy configuration
- Removed redundant handler-level `trusted_proxies` (server-level configuration already provides global
  IP spoofing protection)
- Fixed proxy host save failure (500 error) when updating enable_standard_headers, forward_auth_enabled,
  or waf_disabled fields
- Fixed auth pass-through failure for Seerr/Overseerr caused by missing standard proxy headers

### Security

- **Trusted Proxies**: Caddy configuration now always includes `trusted_proxies` directive when proxy
  headers are enabled, preventing IP spoofing attacks by ensuring headers are only trusted from Charon
  itself

### Migration Guide for Existing Users

Existing proxy hosts will have standard headers **disabled by default** to maintain backward compatibility
with applications that may not expect or handle these headers correctly. To enable standard headers on
existing hosts:

#### Option 1: Enable on individual hosts

1. Navigate to **Proxy Hosts**
2. Click **Edit** on the desired host
3. Scroll to the **Standard Proxy Headers** section
4. Check the **"Enable Standard Proxy Headers"** checkbox
5. Click **Save**

#### Option 2: Bulk enable on multiple hosts

1. Navigate to **Proxy Hosts**
2. Select the checkboxes for hosts you want to update
3. Click the **"Bulk Apply"** button at the top
4. In the **Bulk Apply Settings** modal, find **"Standard Proxy Headers"**
5. Toggle the switch to **ON**
6. Check the **"Apply to selected hosts"** checkbox for this setting
7. Click **"Apply Changes"**

**What do these headers do?**

- **X-Real-IP**: Provides the client's actual IP address (bypasses proxy IP)
- **X-Forwarded-Proto**: Indicates the original protocol (http or https)
- **X-Forwarded-Host**: Contains the original Host header from the client
- **X-Forwarded-Port**: Indicates the original port number used by the client
- **X-Forwarded-For**: Automatically managed by Caddy (shows chain of proxies)

**Why the default changed:**

Most modern web applications expect these headers for proper logging, security, and functionality. New
proxy hosts will have this enabled by default to follow industry best practices.

**When to keep headers disabled:**

- Legacy applications that don't understand proxy headers
- Applications with custom IP detection logic that might conflict
- Security-sensitive applications where you want to control header injection manually
