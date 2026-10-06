package services

import (
	"fmt"

	"gorm.io/gorm"
)

// WithWriteLock runs fn inside a transaction that holds SQLite's write lock
// from the first statement on, so a check-then-write sequence inside fn is
// atomic with respect to other writers.
//
// The lock is taken with a no-op UPDATE on lockModel's table before fn runs.
// SQLite's write lock is database-wide, so the choice of table only needs to
// exist; it does not restrict which tables fn may touch. Concurrent callers
// block on the lock (bounded by the connection's busy_timeout) and then
// observe the committed result of the winner. A transaction was chosen over
// a process-level mutex because it also serializes against other connections
// and processes, and costs nothing extra on the single-connection pool.
//
// fn must use the supplied tx for every query: on a single-connection pool a
// query issued through the outer handle would wait for the connection that
// this transaction is holding.
func WithWriteLock(db *gorm.DB, lockModel any, fn func(tx *gorm.DB) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(lockModel).Where("1 = 0").UpdateColumn("id", gorm.Expr("id")).Error; err != nil {
			return fmt.Errorf("acquire write lock: %w", err)
		}
		return fn(tx)
	})
}
