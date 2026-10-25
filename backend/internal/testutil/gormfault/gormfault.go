// Package gormfault makes chosen GORM statements fail, so tests can reach the error branches of
// database code that a healthy in-memory SQLite database never takes.
//
// A fault is a GORM callback registered before the statement runs. It sets an error on the statement,
// which makes GORM skip the statement and return the error to the caller.
package gormfault

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Op selects the GORM callback chain a fault is registered on.
type Op int

const (
	// Query covers First, Find, Count and Scan.
	Query Op = iota
	// Update covers Update and Updates.
	Update
	// Delete covers Delete.
	Delete
)

var opNames = [...]string{Query: "query", Update: "update", Delete: "delete"}

func (o Op) String() string {
	return opNames[o]
}

var faultSeq atomic.Uint64

// Inject makes every statement on table that match fails. A nil match matches all statements on the table.
func Inject(t testing.TB, db *gorm.DB, op Op, table string, match func(tx *gorm.DB) bool) {
	t.Helper()
	InjectNth(t, db, op, table, 0, match)
}

// InjectNth makes only the nth (1-based) matching statement on table fail. Earlier and later matching
// statements run normally. n == 0 fails every matching statement, like Inject.
func InjectNth(t testing.TB, db *gorm.DB, op Op, table string, n int, match func(tx *gorm.DB) bool) {
	t.Helper()
	seen := 0
	fn := func(tx *gorm.DB) {
		if tx.Statement.Table != table {
			return
		}
		if match != nil && !match(tx) {
			return
		}
		seen++
		if n != 0 && seen != n {
			return
		}
		_ = tx.AddError(fmt.Errorf("injected %s failure on %s: %w", op, table, errInjected))
	}

	name := fmt.Sprintf("gormfault:%s:%s:%d", op, table, faultSeq.Add(1))
	var err error
	switch op {
	case Query:
		err = db.Callback().Query().Before("gorm:query").Register(name, fn)
	case Update:
		err = db.Callback().Update().Before("gorm:update").Register(name, fn)
	case Delete:
		err = db.Callback().Delete().Before("gorm:delete").Register(name, fn)
	}
	require.NoError(t, err, "gormfault: register %s fault on %s", op, table)
}

// errInjected marks every error this package creates, so tests can tell an injected failure apart.
var errInjected = errors.New("injected for test")

// IsInjected reports whether err was produced by an injected fault.
func IsInjected(err error) bool {
	return errors.Is(err, errInjected)
}

// VarIs matches a statement whose WHERE clause binds v, such as `name = ?` with v as the value.
// GORM fills Statement.Vars only while it builds the SQL, after the fault callback runs, so the
// bound values are read from the clause instead.
func VarIs(v any) func(tx *gorm.DB) bool {
	return func(tx *gorm.DB) bool {
		where, _ := tx.Statement.Clauses["WHERE"].Expression.(clause.Where)
		for _, e := range where.Exprs {
			if bound, ok := e.(clause.Expr); ok {
				for _, got := range bound.Vars {
					if got == v {
						return true
					}
				}
			}
		}
		return false
	}
}

// UpdateSets matches an update whose column map (Update, Updates with a map) contains key.
func UpdateSets(key string) func(tx *gorm.DB) bool {
	return func(tx *gorm.DB) bool {
		switch dest := tx.Statement.Dest.(type) {
		case map[string]any:
			_, ok := dest[key]
			return ok
		default:
			return false
		}
	}
}
