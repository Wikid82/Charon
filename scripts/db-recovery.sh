#!/usr/bin/env bash
# ==============================================================================
# Charon Database Recovery Script
# ==============================================================================
# This script performs database integrity checks and recovery operations for
# the Charon SQLite database. It can detect corruption, create backups, and
# attempt to recover data using SQLite's .dump command.
#
# Usage: ./scripts/db-recovery.sh [--force]
#   --force: Skip confirmation prompts
#

# ⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
#    Please use: .github/skills/scripts/skill-runner.sh utility-db-recovery
#    For more info: docs/AGENT_SKILLS_MIGRATION.md
echo "⚠️  WARNING: This script is deprecated and will be removed in v2.0.0" >&2
echo "    Please use: .github/skills/scripts/skill-runner.sh utility-db-recovery" >&2
echo "    For more info: docs/AGENT_SKILLS_MIGRATION.md" >&2
echo "" >&2
sleep 1
# Exit codes:
#   0 - Success (database healthy or recovered)
#   1 - Failure (recovery failed or prerequisites missing)
# ==============================================================================

set -euo pipefail

# Configuration
DOCKER_DB_PATH="/app/data/charon.db"
LOCAL_DB_PATH="backend/data/charon.db"
BACKUP_DIR=""
DB_PATH=""
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
FORCE_MODE=false

# Colors for output (disabled if not a terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# ==============================================================================
# Helper Functions
# ==============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if sqlite3 is available
check_prerequisites() {
    if ! command -v sqlite3 &> /dev/null; then
        log_error "sqlite3 is not installed or not in PATH"
        log_info "Install with: apt-get install sqlite3 (Debian/Ubuntu)"
        log_info "          or: apk add sqlite (Alpine)"
        log_info "          or: brew install sqlite (macOS)"
        exit 1
    fi
    log_info "sqlite3 found: $(sqlite3 --version)"
}

# Detect environment (Docker vs Local)
detect_environment() {
    if [ -f "$DOCKER_DB_PATH" ]; then
        DB_PATH="$DOCKER_DB_PATH"
        BACKUP_DIR="/app/data/backups"
        log_info "Running in Docker environment"
    elif [ -f "$LOCAL_DB_PATH" ]; then
        DB_PATH="$LOCAL_DB_PATH"
        BACKUP_DIR="backend/data/backups"
        log_info "Running in local development environment"
    else
        log_error "Database not found at expected locations:"
        log_error "  - Docker: $DOCKER_DB_PATH"
        log_error "  - Local:  $LOCAL_DB_PATH"
        exit 1
    fi
    log_info "Database path: $DB_PATH"
}

# Create backup directory if it doesn't exist
ensure_backup_dir() {
    if [ ! -d "$BACKUP_DIR" ]; then
        mkdir -p "$BACKUP_DIR"
        log_info "Created backup directory: $BACKUP_DIR"
    fi
}

# Create a timestamped backup of the current database
create_backup() {
    local backup_file="${BACKUP_DIR}/charon_backup_${TIMESTAMP}.db"

    log_info "Creating backup: $backup_file"
    cp "$DB_PATH" "$backup_file"

    # Also backup WAL and SHM files if they exist
    if [ -f "${DB_PATH}-wal" ]; then
        cp "${DB_PATH}-wal" "${backup_file}-wal"
        log_info "Backed up WAL file"
    fi
    if [ -f "${DB_PATH}-shm" ]; then
        cp "${DB_PATH}-shm" "${backup_file}-shm"
        log_info "Backed up SHM file"
    fi

    log_success "Backup created successfully"
    echo "$backup_file"
}

# Run SQLite integrity check
run_integrity_check() {
    log_info "Running SQLite integrity check..."

    local result
    result=$(sqlite3 "$DB_PATH" "PRAGMA integrity_check;" 2>&1) || true

    echo "$result"

    if [ "$result" = "ok" ]; then
        return 0
    else
        return 1
    fi
}

# Attempt to recover database using .dump
recover_database() {
    local dump_file="${BACKUP_DIR}/charon_dump_${TIMESTAMP}.sql"
    local recovered_db="${BACKUP_DIR}/charon_recovered_${TIMESTAMP}.db"

    log_info "Attempting database recovery..."

    # Export database using .dump (works even with some corruption)
    log_info "Exporting database via .dump command..."
    if ! sqlite3 "$DB_PATH" ".dump" > "$dump_file" 2>&1; then
        log_error "Failed to export database dump"
        return 1
    fi
    log_success "Database dump created: $dump_file"

    # Check if dump file has content
    if [ ! -s "$dump_file" ]; then
        log_error "Dump file is empty - no data to recover"
        return 1
    fi

    # Create new database from dump
    log_info "Creating new database from dump..."
    if ! sqlite3 "$recovered_db" < "$dump_file" 2>&1; then
        log_error "Failed to create database from dump"
        return 1
    fi
    log_success "Recovered database created: $recovered_db"

    # Verify recovered database integrity
    log_info "Verifying recovered database integrity..."
    local verify_result
    verify_result=$(sqlite3 "$recovered_db" "PRAGMA integrity_check;" 2>&1) || true
    if [ "$verify_result" != "ok" ]; then
        log_error "Recovered database failed integrity check"
        log_error "Result: $verify_result"
        return 1
    fi
    log_success "Recovered database passed integrity check"

    # Replace original with recovered database
    log_info "Replacing original database with recovered version..."

    # Remove old WAL/SHM files first
    rm -f "${DB_PATH}-wal" "${DB_PATH}-shm"

    # Move recovered database to original location
    mv "$recovered_db" "$DB_PATH"
    log_success "Database replaced successfully"

    return 0
}

# Enable WAL mode on database
enable_wal_mode() {
    log_info "Enabling WAL (Write-Ahead Logging) mode..."

    local current_mode
    current_mode=$(sqlite3 "$DB_PATH" "PRAGMA journal_mode;" 2>&1) || true

    if [ "$current_mode" = "wal" ]; then
        log_info "WAL mode already enabled"
        return 0
    fi

    if sqlite3 "$DB_PATH" "PRAGMA journal_mode=WAL;" > /dev/null 2>&1; then
        log_success "WAL mode enabled"
        return 0
    else
        log_warn "Failed to enable WAL mode (database may be locked)"
        return 1
    fi
}

# Cleanup old backups (keep last 10)
cleanup_old_backups() {
    log_info "Cleaning up old backups (keeping last 10)..."

    local backup_count
    backup_count=$(find "$BACKUP_DIR" -name "charon_backup_*.db" -type f 2>/dev/null | wc -l)

    if [ "$backup_count" -gt 10 ]; then
        find "$BACKUP_DIR" -name "charon_backup_*.db" -type f -printf '%T@ %p\n' 2>/dev/null | \
            sort -n | head -n -10 | cut -d' ' -f2- | \
            while read -r file; do
                rm -f "$file" "${file}-wal" "${file}-shm"
                log_info "Removed old backup: $file"
            done
    fi
}

# Parse command line arguments
parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --force|-f)
                FORCE_MODE=true
                shift
                ;;
            --help|-h)
                echo "Usage: $0 [--force]"
                echo ""
                echo "Options:"
                echo "  --force, -f    Skip confirmation prompts"
                echo "  --help, -h     Show this help message"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
}

# ==============================================================================
# Main Script
# ==============================================================================

main() {
    echo "=============================================="
    echo "  Charon Database Recovery Tool"
    echo "=============================================="
    echo ""

    parse_args "$@"

    # Step 1: Check prerequisites
    check_prerequisites

    # Step 2: Detect environment
    detect_environment

    # Step 3: Ensure backup directory exists
    ensure_backup_dir

    # Step 4: Create backup before any operations
    local backup_file
    backup_file=$(create_backup)
    echo ""

    # Step 5: Run integrity check
    echo "=============================================="
    echo "  Integrity Check Results"
    echo "=============================================="
    local integrity_result
    if integrity_result=$(run_integrity_check); then
        echo "$integrity_result"
        log_success "Database integrity check passed!"
        echo ""

        # Even if healthy, ensure WAL mode is enabled
        enable_wal_mode

        # Cleanup old backups
        cleanup_old_backups

        echo ""
        echo "=============================================="
        echo "  Summary"
        echo "=============================================="
        log_success "Database is healthy"
        log_info "Backup stored at: $backup_file"
        exit 0
    fi

    # Database has issues
    echo "$integrity_result"
    log_error "Database integrity check FAILED"
    echo ""

    # Step 6: Confirm recovery (unless force mode)
    if [ "$FORCE_MODE" != "true" ]; then
        echo -e "${YELLOW}WARNING: Database corruption detected!${NC}"
        echo "This script will attempt to recover the database."
        echo "A backup has already been created at: $backup_file"
        echo ""
        read -p "Continue with recovery? (y/N): " -r confirm
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            log_info "Recovery cancelled by user"
            exit 1
        fi
    fi

    # Step 7: Attempt recovery
    echo ""
    echo "=============================================="
    echo "  Recovery Process"
    echo "=============================================="
    if recover_database; then
        # Step 8: Enable WAL mode on recovered database
        enable_wal_mode

        # Cleanup old backups
        cleanup_old_backups

        echo ""
        echo "=============================================="
        echo "  Summary"
        echo "=============================================="
        log_success "Database recovery completed successfully!"
        log_info "Original backup: $backup_file"
        log_info "Please restart the Charon application"
        exit 0
    else
        echo ""
        echo "=============================================="
        echo "  Summary"
        echo "=============================================="
        log_error "Database recovery FAILED"
        log_info "Your original database backup is at: $backup_file"
        log_info "SQL dump (if created) is in: $BACKUP_DIR"
        log_info "Manual intervention may be required"
        exit 1
    fi
}

# Run main function with all arguments
main "$@"
