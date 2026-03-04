#!/bin/sh
# Register the Caddy bouncer with CrowdSec LAPI
# This script is idempotent - safe to run multiple times
# POSIX-compatible - do not use bash-specific syntax

set -e

BOUNCER_NAME="${CROWDSEC_BOUNCER_NAME:-caddy-bouncer}"
API_KEY_FILE="/etc/crowdsec/bouncers/${BOUNCER_NAME}.key"

# Ensure bouncer directory exists
mkdir -p /etc/crowdsec/bouncers

# Check if bouncer already registered
if cscli bouncers list 2>/dev/null | grep -q "${BOUNCER_NAME}"; then
    echo "Bouncer '${BOUNCER_NAME}' already registered"

    # If key file exists, use it
    if [ -f "$API_KEY_FILE" ]; then
        echo "Using existing API key from ${API_KEY_FILE}"
        cat "$API_KEY_FILE"
        exit 0
    fi

    # Key file missing but bouncer registered - re-register
    echo "API key file missing, re-registering bouncer..."
    cscli bouncers delete "${BOUNCER_NAME}" 2>/dev/null || true
fi

# Register new bouncer and capture API key
echo "Registering bouncer '${BOUNCER_NAME}'..."
API_KEY=$(cscli bouncers add "${BOUNCER_NAME}" -o raw 2>/dev/null)

if [ -z "$API_KEY" ]; then
    echo "ERROR: Failed to register bouncer" >&2
    exit 1
fi

# Save API key to file
echo "$API_KEY" > "$API_KEY_FILE"
chmod 600 "$API_KEY_FILE"

echo "Bouncer registered successfully"
echo "$API_KEY"
