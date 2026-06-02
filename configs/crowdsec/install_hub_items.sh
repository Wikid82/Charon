#!/bin/sh
# Install required CrowdSec hub items (parsers, scenarios, collections)
# This script runs during container startup
# POSIX-compatible - do not use bash-specific syntax

set -e

echo "Installing CrowdSec hub items for Charon..."

# Hub index update is handled by the entrypoint before this script is called.
# Do not duplicate it here — a redundant update adds ~3s to startup for no benefit.

# Helper: only install if not already present (avoids 5-10s per cscli call on rebuilds)
install_if_missing() {
    type="$1"   # parsers | scenarios | collections
    name="$2"
    label="${3:-$name}"
    if cscli "${type}" inspect "${name}" >/dev/null 2>&1; then
        echo "  ✓ ${label} already installed, skipping"
    else
        echo "  Installing ${label}..."
        cscli "${type}" install "${name}" || echo "⚠️ Failed to install ${name}"
    fi
}

# Install Caddy log parser (if available)
# Note: crowdsecurity/caddy-logs may not exist yet - check hub
if cscli parsers inspect crowdsecurity/caddy-logs >/dev/null 2>&1; then
    echo "Installing Caddy log parser..."
    install_if_missing parsers crowdsecurity/caddy-logs "Caddy log parser"
else
    echo "Caddy-specific parser not available, using HTTP parser..."
fi

# Install base HTTP parsers (always needed)
echo "Installing base parsers..."
install_if_missing parsers crowdsecurity/http-logs "http-logs"
install_if_missing parsers crowdsecurity/syslog-logs "syslog-logs"
install_if_missing parsers crowdsecurity/geoip-enrich "geoip-enrich"
install_if_missing parsers crowdsecurity/whitelists "whitelists"

# Install HTTP scenarios for attack detection
echo "Installing HTTP scenarios..."
install_if_missing scenarios crowdsecurity/http-probing "http-probing"
install_if_missing scenarios crowdsecurity/http-sensitive-files "http-sensitive-files"
install_if_missing scenarios crowdsecurity/http-backdoors-attempts "http-backdoors-attempts"
install_if_missing scenarios crowdsecurity/http-path-traversal-probing "http-path-traversal-probing"
install_if_missing scenarios crowdsecurity/http-xss-probing "http-xss-probing"
install_if_missing scenarios crowdsecurity/http-sqli-probing "http-sqli-probing"
install_if_missing scenarios crowdsecurity/http-generic-bf "http-generic-bf"

# Install CVE collection for known vulnerabilities
echo "Installing CVE collection..."
install_if_missing collections crowdsecurity/http-cve "http-cve collection"

# Install base HTTP collection (bundles common scenarios)
echo "Installing base HTTP collection..."
install_if_missing collections crowdsecurity/base-http-scenarios "base-http-scenarios collection"

# Install Caddy collection (parser + scenarios for Caddy access logs)
echo "Installing Caddy collection..."
install_if_missing collections crowdsecurity/caddy "caddy collection"

# Verify installation
echo ""
echo "=== Installed Components ==="
echo "Parsers:"
cscli parsers list 2>/dev/null | head -15 || echo "  (unable to list)"

echo ""
echo "Scenarios:"
cscli scenarios list 2>/dev/null | head -15 || echo "  (unable to list)"

echo ""
echo "Collections:"
cscli collections list 2>/dev/null | head -10 || echo "  (unable to list)"

echo ""
echo "Hub installation complete!"
