package database

import (
	"fmt"

	"gorm.io/gorm"
)

// secureDeletePragmas maps the readable secure_delete modes (0 off, 1 on,
// 2 fast) to the statement that sets them. PRAGMA cannot take bound values.
var secureDeletePragmas = [...]string{
	"PRAGMA secure_delete = 0",
	"PRAGMA secure_delete = 1",
	"PRAGMA secure_delete = 2",
}

// WithSecureDelete runs fn on one pinned connection with secure_delete switched on, so overwritten pages are
// scrubbed, and checkpoints the WAL after fn succeeds so the old bytes do not linger there. The previous
// secure_delete mode is restored on every path, including errors. fn must do its writes through the connection
// it receives.
func WithSecureDelete(db *gorm.DB, fn func(conn *gorm.DB) error) error {
	return db.Connection(func(conn *gorm.DB) (err error) {
		var previous int
		previous, err = readSecureDelete(conn)
		if err != nil {
			return err
		}
		if err = conn.Exec(secureDeletePragmas[1]).Error; err != nil {
			return fmt.Errorf("enabling secure_delete: %w", err)
		}
		defer func() {
			if restoreErr := conn.Exec(secureDeletePragmas[previous]).Error; restoreErr != nil && err == nil {
				err = fmt.Errorf("restoring secure_delete: %w", restoreErr)
			}
		}()

		if fnErr := fn(conn); fnErr != nil {
			return fnErr
		}
		if err = conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
			return fmt.Errorf("checkpointing WAL after secure delete: %w", err)
		}
		return nil
	})
}

// readSecureDelete returns the connection's current secure_delete mode as an
// index into secureDeletePragmas.
func readSecureDelete(conn *gorm.DB) (int, error) {
	var mode int
	if err := conn.Raw("PRAGMA secure_delete").Scan(&mode).Error; err != nil {
		return 0, fmt.Errorf("reading secure_delete: %w", err)
	}
	return validSecureDeleteMode(mode)
}

// validSecureDeleteMode returns mode when it indexes secureDeletePragmas.
func validSecureDeleteMode(mode int) (int, error) {
	if mode < 0 || mode >= len(secureDeletePragmas) {
		return 0, fmt.Errorf("unexpected secure_delete mode %d", mode)
	}
	return mode, nil
}
