#!/usr/bin/env bash
# GORM Security Scanner v1.0.0
# Detects GORM security issues and common mistakes

set -euo pipefail

# Color codes
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Configuration
MODE="${1:---report}"
VERBOSE="${VERBOSE:-0}"
SCAN_DIR="backend"

# State
ISSUES_FOUND=0
CRITICAL_COUNT=0
HIGH_COUNT=0
MEDIUM_COUNT=0
INFO_COUNT=0
SUPPRESSED_COUNT=0
FILES_SCANNED=0
LINES_PROCESSED=0
START_TIME=$(date +%s)

# Exit codes
EXIT_SUCCESS=0
EXIT_ISSUES_FOUND=1
EXIT_INVALID_ARGS=2
EXIT_FS_ERROR=3

# Helper Functions
log_debug() {
    if [[ $VERBOSE -eq 1 ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $*" >&2
    fi
}

log_warning() {
    echo -e "${YELLOW}⚠️  WARNING:${NC} $*" >&2
}

log_error() {
    echo -e "${RED}❌ ERROR:${NC} $*" >&2
}

print_header() {
    echo -e "${BOLD}🔍 GORM Security Scanner v1.0.0${NC}"
    echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

print_summary() {
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))

    echo ""
    echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${BOLD}📊 SUMMARY${NC}"
    echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "  Scanned: $FILES_SCANNED Go files ($LINES_PROCESSED lines)"
    echo "  Duration: ${duration} seconds"
    echo ""
    echo -e "  ${RED}🔴 CRITICAL:${NC} $CRITICAL_COUNT issues"
    echo -e "  ${YELLOW}🟡 HIGH:${NC}     $HIGH_COUNT issues"
    echo -e "  ${BLUE}🔵 MEDIUM:${NC}   $MEDIUM_COUNT issues"
    echo -e "  ${GREEN}🟢 INFO:${NC}     $INFO_COUNT suggestions"

    if [[ $SUPPRESSED_COUNT -gt 0 ]]; then
        echo ""
        echo -e "  🔇 Suppressed: $SUPPRESSED_COUNT issues (see --verbose for details)"
    fi

    echo ""
    local total_issues=$((CRITICAL_COUNT + HIGH_COUNT + MEDIUM_COUNT))
    echo "  Total Issues: $total_issues (excluding informational)"
    echo ""
    echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    if [[ $total_issues -gt 0 ]]; then
        echo -e "${RED}❌ FAILED:${NC} $total_issues security issues detected"
        echo ""
        echo "Run './scripts/scan-gorm-security.sh --help' for usage information"
    else
        echo -e "${GREEN}✅ PASSED:${NC} No security issues detected"
    fi
}

has_suppression_comment() {
    local file="$1"
    local line_num="$2"

    # Check for // gorm-scanner:ignore comment on the line or the line before
    local start_line=$((line_num > 1 ? line_num - 1 : line_num))

    if sed -n "${start_line},${line_num}p" "$file" 2>/dev/null | grep -q '//.*gorm-scanner:ignore'; then
        log_debug "Suppression comment found at $file:$line_num"
        : $((SUPPRESSED_COUNT++))
        return 0
    fi

    return 1
}

is_gorm_model() {
    local file="$1"
    local struct_name="$2"

    # Heuristic 1: File in internal/models/ directory
    if [[ "$file" == *"/internal/models/"* ]]; then
        log_debug "$struct_name is in models directory"
        return 0
    fi

    # Heuristic 2: Struct has 2+ fields with gorm: tags
    local gorm_tag_count=$(grep -A 30 "^type $struct_name struct" "$file" 2>/dev/null | grep -c 'gorm:' || true)
    if [[ $gorm_tag_count -ge 2 ]]; then
        log_debug "$struct_name has $gorm_tag_count gorm tags"
        return 0
    fi

    # Heuristic 3: Embeds gorm.Model
    if grep -A 5 "^type $struct_name struct" "$file" 2>/dev/null | grep -q 'gorm\.Model'; then
        log_debug "$struct_name embeds gorm.Model"
        return 0
    fi

    log_debug "$struct_name is not a GORM model"
    return 1
}

report_issue() {
    local severity="$1"
    local code="$2"
    local file="$3"
    local line_num="$4"
    local struct_name="$5"
    local message="$6"
    local fix="$7"

    local color=""
    local emoji=""
    local severity_label=""

    case "$severity" in
        CRITICAL)
            color="$RED"
            emoji="🔴"
            severity_label="CRITICAL"
            : $((CRITICAL_COUNT++))
            ;;
        HIGH)
            color="$YELLOW"
            emoji="🟡"
            severity_label="HIGH"
            : $((HIGH_COUNT++))
            ;;
        MEDIUM)
            color="$BLUE"
            emoji="🔵"
            severity_label="MEDIUM"
            : $((MEDIUM_COUNT++))
            ;;
        INFO)
            color="$GREEN"
            emoji="🟢"
            severity_label="INFO"
            : $((INFO_COUNT++))
            ;;
    esac

    : $((ISSUES_FOUND++))

    echo ""
    echo -e "${color}${emoji} ${severity_label}: ${message}${NC}"
    echo -e "  📄 File: ${file}:${line_num}"
    echo -e "  🏗️  Struct: ${struct_name}"
    echo ""
    echo -e "  ${fix}"
    echo ""
}

detect_id_leak() {
    log_debug "Running Pattern 1: ID Leak Detection"

    # Use process substitution instead of pipe to avoid subshell issues
    while IFS= read -r file; do
        [[ -z "$file" ]] && continue

        : $((FILES_SCANNED++))
        local line_count=$(wc -l < "$file" 2>/dev/null || echo 0)
        : $((LINES_PROCESSED+=line_count))

        log_debug "Scanning $file"

        # Look for ID fields with numeric types that have json:"id" and gorm primaryKey
        while IFS=: read -r line_num line_content; do
            # Skip if not a field definition (e.g., inside comments or other contexts)
            if ! echo "$line_content" | grep -E '^\s*(ID|Id)\s+\*?(u?int|int64)' >/dev/null; then
                continue
            fi

            # Check if has both json:"id" and gorm primaryKey
            if echo "$line_content" | grep 'json:"id"' >/dev/null && \
               echo "$line_content" | grep -iE 'gorm:"[^"]*primarykey' >/dev/null; then

                # Check for suppression
                if has_suppression_comment "$file" "$line_num"; then
                    continue
                fi

                # Get struct name by looking backwards
                local struct_name=$(awk -v line="$line_num" 'NR<line && /^type .* struct/ {name=$2} END {print name}' "$file")

                if [[ -z "$struct_name" ]]; then
                    struct_name="Unknown"
                fi

                report_issue "CRITICAL" "ID-LEAK" "$file" "$line_num" "$struct_name" \
                    "GORM Model ID Field Exposed in JSON" \
                    "💡 Fix: Change json:\"id\" to json:\"-\" and use UUID field for external references"
            fi
        done < <(grep -n 'ID.*uint\|ID.*int64\|ID.*int[^6]' "$file" 2>/dev/null || true)
    done < <(find "$SCAN_DIR/internal/models" -name "*.go" -type f 2>/dev/null || true)
}

detect_dto_embedding() {
    log_debug "Running Pattern 2: DTO Embedding Detection"

    # Scan handlers and services for Response/DTO structs
    local scan_dirs="$SCAN_DIR/internal/api/handlers $SCAN_DIR/internal/services"

    for dir in $scan_dirs; do
        if [[ ! -d "$dir" ]]; then
            continue
        fi

        while IFS= read -r file; do
            [[ -z "$file" ]] && continue

            # Look for Response/DTO structs with embedded models
            while IFS=: read -r line_num line_content; do
                local struct_name=$(echo "$line_content" | sed 's/^type \([^ ]*\) struct.*/\1/')

                # Check next 20 lines for embedded models
                local struct_body=$(sed -n "$((line_num+1)),$((line_num+20))p" "$file" 2>/dev/null)

                if echo "$struct_body" | grep -E '^\s+models\.[A-Z]' >/dev/null; then
                    local embedded_line=$(echo "$struct_body" | grep -n -E '^\s+models\.[A-Z]' | head -1 | cut -d: -f1)
                    local actual_line=$((line_num + embedded_line))

                    if has_suppression_comment "$file" "$actual_line"; then
                        continue
                    fi

                    report_issue "HIGH" "DTO-EMBED" "$file" "$actual_line" "$struct_name" \
                        "Response DTO Embeds Model" \
                        "💡 Fix: Explicitly define response fields instead of embedding the model"
                fi
            done < <(grep -n 'type.*\(Response\|DTO\).*struct' "$file" 2>/dev/null || true)
        done < <(find "$dir" -name "*.go" -type f 2>/dev/null || true)
    done
}

detect_exposed_secrets() {
    log_debug "Running Pattern 5: Exposed API Keys/Secrets Detection"

    # Only scan model files for this pattern
    while IFS= read -r file; do
        [[ -z "$file" ]] && continue

        # Find fields with sensitive names that don't have json:"-"
        while IFS=: read -r line_num line_content; do
            # Skip if already has json:"-"
            if echo "$line_content" | grep 'json:"-"' >/dev/null; then
                continue
            fi

            # Skip if no json tag at all (might be internal-only field)
            if ! echo "$line_content" | grep 'json:' >/dev/null; then
                continue
            fi

            # Check for suppression
            if has_suppression_comment "$file" "$line_num"; then
                continue
            fi

            local struct_name=$(awk -v line="$line_num" 'NR<line && /^type .* struct/ {name=$2} END {print name}' "$file")
            local field_name=$(echo "$line_content" | awk '{print $1}')

            report_issue "CRITICAL" "SECRET-LEAK" "$file" "$line_num" "${struct_name:-Unknown}" \
                "Sensitive Field '$field_name' Exposed in JSON" \
                "💡 Fix: Change json tag to json:\"-\" to hide sensitive data"
        done < <(grep -n -iE '(APIKey|Secret|Token|Password|Hash)\s+string' "$file" 2>/dev/null || true)
    done < <(find "$SCAN_DIR/internal/models" -name "*.go" -type f 2>/dev/null || true)
}

detect_missing_primary_key() {
    log_debug "Running Pattern 3: Missing Primary Key Tag Detection"

    while IFS= read -r file; do
        [[ -z "$file" ]] && continue

        # Look for ID fields with gorm tag but no primaryKey
        while IFS=: read -r line_num line_content; do
            # Skip if has primaryKey
            if echo "$line_content" | grep -iE 'gorm:"[^"]*primarykey' >/dev/null; then
                continue
            fi

            # Skip if doesn't have gorm tag
            if ! echo "$line_content" | grep 'gorm:' >/dev/null; then
                continue
            fi

            if has_suppression_comment "$file" "$line_num"; then
                continue
            fi

            local struct_name=$(awk -v line="$line_num" 'NR<line && /^type .* struct/ {name=$2} END {print name}' "$file")

            report_issue "MEDIUM" "MISSING-PK" "$file" "$line_num" "${struct_name:-Unknown}" \
                "ID Field Missing Primary Key Tag" \
                "💡 Fix: Add 'primaryKey' to gorm tag: gorm:\"primaryKey\""
        # Only match primary key ID field (not foreign keys like CertificateID, AccessListID, etc.)
        done < <(grep -n -E '^\s+ID\s+' "$file" 2>/dev/null || true)
    done < <(find "$SCAN_DIR/internal/models" -name "*.go" -type f 2>/dev/null || true)
}

detect_foreign_key_index() {
    log_debug "Running Pattern 4: Foreign Key Index Detection"

    while IFS= read -r file; do
        [[ -z "$file" ]] && continue

        # Find fields ending with ID that have gorm tag but no index
        while IFS=: read -r line_num line_content; do
            # Skip primary key
            if echo "$line_content" | grep -E '^\s+ID\s+' >/dev/null; then
                continue
            fi

            # Skip if has index
            if echo "$line_content" | grep -E 'gorm:"[^"]*index' >/dev/null; then
                continue
            fi

            if has_suppression_comment "$file" "$line_num"; then
                continue
            fi

            local struct_name=$(awk -v line="$line_num" 'NR<line && /^type .* struct/ {name=$2} END {print name}' "$file")
            local field_name=$(echo "$line_content" | awk '{print $1}')

            report_issue "INFO" "MISSING-INDEX" "$file" "$line_num" "${struct_name:-Unknown}" \
                "Foreign Key '$field_name' Missing Index" \
                "💡 Suggestion: Add gorm:\"index\" for better query performance"
        done < <(grep -n -E '\s+[A-Z][a-zA-Z]*ID\s+\*?uint.*gorm:' "$file" 2>/dev/null || true)
    done < <(find "$SCAN_DIR/internal/models" -name "*.go" -type f 2>/dev/null || true)
}

detect_missing_uuid() {
    log_debug "Running Pattern 6: Missing UUID Detection"

    # This pattern is complex and less critical, skip for now to improve performance
    log_debug "Pattern 6 skipped for performance (can be enabled later)"
}

show_help() {
    cat << EOF
GORM Security Scanner v1.0.0
Detects GORM security issues and common mistakes

USAGE:
    $0 [MODE] [OPTIONS]

MODES:
    --report    Report all issues but always exit 0 (default)
    --check     Report issues and exit 1 if any found
    --enforce   Same as --check (block on issues)

OPTIONS:
    --help      Show this help message
    --verbose   Enable verbose debug output

ENVIRONMENT:
    VERBOSE=1   Enable debug logging

EXAMPLES:
    # Report mode (no failure)
    $0 --report

    # Check mode (fails if issues found)
    $0 --check

    # Verbose output
    VERBOSE=1 $0 --report

EXIT CODES:
    0 - Success (report mode) or no issues (check/enforce mode)
    1 - Issues found (check/enforce mode)
    2 - Invalid arguments
    3 - File system error

For more information, see: docs/plans/gorm_security_scanner_spec.md
EOF
}

# Main execution
main() {
    # Parse arguments
    case "${MODE}" in
        --help|-h)
            show_help
            exit 0
            ;;
        --report)
            ;;
        --check|--enforce)
            ;;
        *)
            log_error "Invalid mode: $MODE"
            show_help
            exit $EXIT_INVALID_ARGS
            ;;
    esac

    # Check if scan directory exists
    if [[ ! -d "$SCAN_DIR" ]]; then
        log_error "Scan directory not found: $SCAN_DIR"
        exit $EXIT_FS_ERROR
    fi

    print_header
    echo "📂 Scanning: $SCAN_DIR/"
    echo ""

    # Run all detection patterns
    detect_id_leak
    detect_dto_embedding
    detect_exposed_secrets
    detect_missing_primary_key
    detect_foreign_key_index
    detect_missing_uuid

    print_summary

    # Exit based on mode
    local total_issues=$((CRITICAL_COUNT + HIGH_COUNT + MEDIUM_COUNT))

    if [[ "$MODE" == "--report" ]]; then
        exit $EXIT_SUCCESS
    elif [[ $total_issues -gt 0 ]]; then
        exit $EXIT_ISSUES_FOUND
    else
        exit $EXIT_SUCCESS
    fi
}

main "$@"
