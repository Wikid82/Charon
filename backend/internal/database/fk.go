package database

import (
	"errors"
	"strings"
)

// sqliteForeignKeyConstraintCode is SQLITE_CONSTRAINT_FOREIGNKEY (787), the extended result code SQLite reports when
// a statement breaks a reference between rows.
const sqliteForeignKeyConstraintCode = 787

// IsForeignKeyViolation reports whether err is a reference violation reported by SQLite. The pure-Go driver exposes
// the extended result code; the cgo driver used by tests only reports the message.
func IsForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	var coded interface{ Code() int }
	if errors.As(err, &coded) && coded.Code() == sqliteForeignKeyConstraintCode {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "foreign key constraint failed")
}
