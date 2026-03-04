#!/bin/sh
# Install required CrowdSec hub items (parsers, scenarios, collections)
# This script runs during container startup
# POSIX-compatible - do not use bash-specific syntax

set -e

echo "Installing CrowdSec hub items for Charon..."

# Update hub index first
echo "Updating hub index..."
cscli hub update 2>/dev/null || echo "Warning: Failed to update hub index"

# Install Caddy log parser (if available)
# Note: crowdsecurity/caddy-logs may not exist yet - check hub
if cscli parsers inspect crowdsecurity/caddy-logs >/dev/null 2>&1; then
    echo "Installing Caddy log parser..."
    cscli parsers install crowdsecurity/caddy-logs --force 2>/dev/null || true
else
    echo "Caddy-specific parser not available, using HTTP parser..."
fi

# Install base HTTP parsers (always needed)
echo "Installing base parsers..."
cscli parsers install crowdsecurity/http-logs --force 2>/dev/null || true
cscli parsers install crowdsecurity/syslog-logs --force 2>/dev/null || true
cscli parsers install crowdsecurity/geoip-enrich --force 2>/dev/null || true

# Install HTTP scenarios for attack detection
echo "Installing HTTP scenarios..."
cscli scenarios install crowdsecurity/http-probing --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-sensitive-files --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-backdoors-attempts --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-path-traversal-probing --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-xss-probing --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-sqli-probing --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-generic-bf --force 2>/dev/null || true

# Install CVE collection for known vulnerabilities
echo "Installing CVE collection..."
cscli collections install crowdsecurity/http-cve --force 2>/dev/null || true

# Install base HTTP collection (bundles common scenarios)
echo "Installing base HTTP collection..."
cscli collections install crowdsecurity/base-http-scenarios --force 2>/dev/null || true

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
