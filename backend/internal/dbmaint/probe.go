package dbmaint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrWriterBusy means another writer holds the database.
var ErrWriterBusy = errors.New("another writer holds the database")

// isWriterBusy recognizes SQLITE_BUSY in a driver error.
func isWriterBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
}

// ProbeWriterLock checks, on the pinned connection, that no other writer holds
// the database: BEGIN EXCLUSIVE with busy_timeout=0, then ROLLBACK. In WAL mode
// readers do not block it, so only writers are detected (ErrWriterBusy). The
// connection's busy_timeout is restored afterwards.
func ProbeWriterLock(ctx context.Context, conn *sql.Conn) (retErr error) {
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout=0"); err != nil {
		return fmt.Errorf("disable busy timeout: %w", err)
	}
	defer func() {
		restoreCtx := context.WithoutCancel(ctx)
		if _, restoreErr := conn.ExecContext(restoreCtx, fmt.Sprintf("PRAGMA busy_timeout=%d", busyTimeoutMillis)); restoreErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("restore busy timeout: %w", restoreErr))
		}
	}()

	if _, err := conn.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		if isWriterBusy(err) {
			return ErrWriterBusy
		}
		return fmt.Errorf("probe writer lock: %w", err)
	}
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); err != nil {
		return fmt.Errorf("release writer lock probe: %w", err)
	}
	return nil
}
