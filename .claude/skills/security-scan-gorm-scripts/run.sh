#!/usr/bin/env bash
# GORM Security Scanner - Skill Runner Wrapper
# Executes the GORM security scanner from the skills framework

set -euo pipefail

# Get the workspace root directory (from skills/security-scan-gorm-scripts/ to project root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Check if scan-gorm-security.sh exists
SCANNER_SCRIPT="${WORKSPACE_ROOT}/scripts/scan-gorm-security.sh"

if [[ ! -f "$SCANNER_SCRIPT" ]]; then
    echo "❌ ERROR: GORM security scanner not found at: $SCANNER_SCRIPT" >&2
    echo "   Ensure the scanner script exists and has execute permissions." >&2
    exit 1
fi

# Make script executable if needed
if [[ ! -x "$SCANNER_SCRIPT" ]]; then
    chmod +x "$SCANNER_SCRIPT"
fi

# Parse arguments
MODE="${1:---report}"
OUTPUT_FILE="${2:-}"

# Validate mode
case "$MODE" in
    --report|--check|--enforce)
        # Valid mode
        ;;
    *)
        echo "❌ ERROR: Invalid mode: $MODE" >&2
        echo "   Valid modes: --report, --check, --enforce" >&2
        echo "" >&2
        echo "Usage: $0 [mode] [output_file]" >&2
        echo "  mode: --report (show all issues, exit 0)" >&2
        echo "        --check  (show issues, exit 1 if found)" >&2
        echo "        --enforce (same as --check)" >&2
        echo "  output_file: Optional path to save report (e.g., gorm-scan.txt)" >&2
        exit 2
        ;;
esac

# Change to workspace root
cd "$WORKSPACE_ROOT"

# Ensure docs/reports directory exists if output file specified
if [[ -n "$OUTPUT_FILE" ]]; then
    OUTPUT_DIR="$(dirname "$OUTPUT_FILE")"
    if [[ "$OUTPUT_DIR" != "." && ! -d "$OUTPUT_DIR" ]]; then
        mkdir -p "$OUTPUT_DIR"
    fi
fi

# Execute the scanner with the specified mode
if [[ -n "$OUTPUT_FILE" ]]; then
    # Save to file and display to console
    "$SCANNER_SCRIPT" "$MODE" | tee "$OUTPUT_FILE"
    EXIT_CODE=${PIPESTATUS[0]}

    echo ""
    echo "📄 Report saved to: $OUTPUT_FILE"
    exit $EXIT_CODE
else
    # Direct execution without file output
    exec "$SCANNER_SCRIPT" "$MODE"
fi
