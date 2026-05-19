#!/bin/sh
# Install required CrowdSec hub items (parsers, scenarios, collections)
# This script runs during container startup
# POSIX-compatible - do not use bash-specific syntax

set -e

echo "Installing CrowdSec hub items for Charon..."

# Hub index update is handled by the entrypoint before this script is called.
# Do not duplicate it here — a redundant update adds ~3s to startup for no benefit.

# Install Caddy log parser (if available)
# Note: crowdsecurity/caddy-logs may not exist yet - check hub
if cscli parsers inspect crowdsecurity/caddy-logs >/dev/null 2>&1; then
    echo "Installing Caddy log parser..."
    cscli parsers install crowdsecurity/caddy-logs --force || echo "⚠️ Failed to install crowdsecurity/caddy-logs"
else
    echo "Caddy-specific parser not available, using HTTP parser..."
fi

# Install base HTTP parsers (always needed)
echo "Installing base parsers..."
cscli parsers install crowdsecurity/http-logs --force || echo "⚠️ Failed to install crowdsecurity/http-logs"
cscli parsers install crowdsecurity/syslog-logs --force || echo "⚠️ Failed to install crowdsecurity/syslog-logs"
timeout 60 cscli parsers install crowdsecurity/geoip-enrich --force || echo "⚠️ Failed or timed out installing crowdsecurity/geoip-enrich (GeoLite2-City.mmdb download may be slow)"
cscli parsers install crowdsecurity/whitelists --force || echo "⚠️  Failed to install crowdsecurity/whitelists"

# Install HTTP scenarios for attack detection
echo "Installing HTTP scenarios..."
cscli scenarios install crowdsecurity/http-probing --force || echo "⚠️ Failed to install crowdsecurity/http-probing"
cscli scenarios install crowdsecurity/http-sensitive-files --force || echo "⚠️ Failed to install crowdsecurity/http-sensitive-files"
cscli scenarios install crowdsecurity/http-backdoors-attempts --force || echo "⚠️ Failed to install crowdsecurity/http-backdoors-attempts"
cscli scenarios install crowdsecurity/http-path-traversal-probing --force || echo "⚠️ Failed to install crowdsecurity/http-path-traversal-probing"
cscli scenarios install crowdsecurity/http-xss-probing --force || echo "⚠️ Failed to install crowdsecurity/http-xss-probing"
cscli scenarios install crowdsecurity/http-sqli-probing --force || echo "⚠️ Failed to install crowdsecurity/http-sqli-probing"
cscli scenarios install crowdsecurity/http-generic-bf --force || echo "⚠️ Failed to install crowdsecurity/http-generic-bf"

# Install CVE collection for known vulnerabilities
echo "Installing CVE collection..."
cscli collections install crowdsecurity/http-cve --force || echo "⚠️ Failed to install crowdsecurity/http-cve"

# Install base HTTP collection (bundles common scenarios)
echo "Installing base HTTP collection..."
cscli collections install crowdsecurity/base-http-scenarios --force || echo "⚠️ Failed to install crowdsecurity/base-http-scenarios"

# Install Caddy collection (parser + scenarios for Caddy access logs)
echo "Installing Caddy collection..."
cscli collections install crowdsecurity/caddy --force || echo "⚠️ Failed to install crowdsecurity/caddy"

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
