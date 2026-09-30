package dbmaint

import (
	"context"
	"database/sql"
	"fmt"
)

// Convert switches the database behind conn to incremental auto-vacuum. SQLite
// can only change the mode of a populated file with a full VACUUM, so this sets
// the pending mode and rebuilds the file in place.
//
// conn must be the pool's only connection (the caller pins it), so nothing else
// writes meanwhile. VACUUM is atomic: a failure, a cancel in the rebuild phase
// or a crash leaves the old database authoritative, and journal_mode=WAL is
// kept. Checkpointing, the in-progress marker and the file-size verification
// belong to Run.
func Convert(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, "PRAGMA auto_vacuum=INCREMENTAL"); err != nil {
		return fmt.Errorf("set auto_vacuum: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	return verifyIncremental(ctx, conn)
}

// verifyIncremental checks the post-condition of a VACUUM that returned nil.
// It ignores cancellation of ctx: VACUUM's copy-back tail is not interruptible,
// so a conversion that finished after a shutdown signal must still be
// recognized as finished, not turned into an error by the cancelled context.
func verifyIncremental(ctx context.Context, q Querier) error {
	mode, err := readPragma(context.WithoutCancel(ctx), q, "auto_vacuum")
	if err != nil {
		return fmt.Errorf("verify auto_vacuum: %w", err)
	}
	if mode != AutoVacuumIncremental {
		return fmt.Errorf("verify auto_vacuum: mode is %d after VACUUM, want %d", mode, AutoVacuumIncremental)
	}
	return nil
}
