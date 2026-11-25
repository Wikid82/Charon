package database

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// Table names of the 12 declared references (see ReferenceRules).
const (
	TableBackupRecords          = "backup_records"
	TableBackupRemoteCopies     = "backup_remote_copies"
	TableDNSProviders           = "dns_providers"
	TableLocations              = "locations"
	TableProxyHosts             = "proxy_hosts"
	TableProxyGroups            = "proxy_groups"
	TableRedirectionHosts       = "redirection_hosts"
	TableRemoteStorageTargets   = "remote_storage_targets"
	TableSSLCertificates        = "ssl_certificates"
	TableSecurityHeaderProfiles = "security_header_profiles"
	TableAccessLists            = "access_lists"
	TableUptimeMonitors         = "uptime_monitors"
	TableUptimeHeartbeats       = "uptime_heartbeats"
)

// RepairAction says what a repair does to a child row whose parent is gone.
type RepairAction int

const (
	// RepairClear sets the reference column to NULL and keeps the row.
	RepairClear RepairAction = iota + 1
	// RepairDeleteRow deletes the child row (and the AlsoDelete rows that point at it).
	RepairDeleteRow
)

// String names the action for log lines.
func (a RepairAction) String() string {
	switch a {
	case RepairClear:
		return "clear"
	case RepairDeleteRow:
		return "delete_row"
	default:
		return "unknown"
	}
}

// DependentRows names rows that point at a deleted child row, removed together with it.
type DependentRows struct{ Table, Column string }

// ReferenceRule describes one declared reference and how a broken value is repaired.
type ReferenceRule struct {
	Child, Column, Parent, ParentColumn string
	Action                              RepairAction
	// AlsoSet holds columns that are set together with a cleared reference (R7/R11 turn the DNS challenge off).
	AlsoSet map[string]any
	// AlsoDelete lists rows removed together with a deleted child row (R12 monitor heartbeats).
	AlsoDelete []DependentRows
}

// ReferenceRules lists every reference the schema declares, one rule each. A test pins that the rule set matches
// the declared references after migration.
var ReferenceRules = []ReferenceRule{
	{Child: TableBackupRemoteCopies, Column: "backup_record_id", Parent: TableBackupRecords, ParentColumn: "id", Action: RepairDeleteRow},
	{Child: TableBackupRemoteCopies, Column: "remote_target_id", Parent: TableRemoteStorageTargets, ParentColumn: "id", Action: RepairDeleteRow},
	{Child: "dns_provider_credentials", Column: "dns_provider_id", Parent: TableDNSProviders, ParentColumn: "id", Action: RepairDeleteRow},
	{Child: TableLocations, Column: "proxy_host_id", Parent: TableProxyHosts, ParentColumn: "id", Action: RepairDeleteRow},
	{Child: TableProxyHosts, Column: "certificate_id", Parent: TableSSLCertificates, ParentColumn: "id", Action: RepairClear},
	{Child: TableProxyHosts, Column: "proxy_group_id", Parent: TableProxyGroups, ParentColumn: "id", Action: RepairClear},
	{Child: TableProxyHosts, Column: "dns_provider_id", Parent: TableDNSProviders, ParentColumn: "id", Action: RepairClear,
		AlsoSet: map[string]any{"use_dns_challenge": false}},
	{Child: TableProxyHosts, Column: "security_header_profile_id", Parent: TableSecurityHeaderProfiles, ParentColumn: "id", Action: RepairClear},
	{Child: TableProxyHosts, Column: "access_list_id", Parent: TableAccessLists, ParentColumn: "id", Action: RepairClear},
	{Child: TableRedirectionHosts, Column: "certificate_id", Parent: TableSSLCertificates, ParentColumn: "id", Action: RepairClear},
	{Child: TableRedirectionHosts, Column: "dns_provider_id", Parent: TableDNSProviders, ParentColumn: "id", Action: RepairClear,
		AlsoSet: map[string]any{"use_dns_challenge": false}},
	{Child: TableUptimeMonitors, Column: "proxy_host_id", Parent: TableProxyHosts, ParentColumn: "id", Action: RepairDeleteRow,
		AlsoDelete: []DependentRows{{Table: TableUptimeHeartbeats, Column: "monitor_id"}}},
}

// errForeignKeysStillOn is returned when reference enforcement cannot be switched off on the pinned connection. That
// only happens inside an open transaction, which is a programming error; boot stops rather than migrate without the
// pause.
var errForeignKeysStillOn = errors.New("database checks could not be paused for the schema update; restart Charon, and report this if it happens again")

// ReferenceViolation is one row that breaks a declared reference, as reported by PRAGMA foreign_key_check.
type ReferenceViolation struct {
	Table  string
	RowID  int64
	Parent string
	FKID   int
}

// RepairResult is what one rule changed: the number of rows and up to maxLoggedIDs of their ids.
type RepairResult struct {
	Rule      ReferenceRule
	Count     int
	SampleIDs []string
}

// maxLoggedIDs caps the row ids written to the log for one rule.
const maxLoggedIDs = 10

// MigrateWithIntegrity runs migrate with reference enforcement paused on one pinned connection, repairs the stored
// rows that break a declared reference, checks the result, and turns enforcement back on. The migration and the
// repair must share that connection: the migration rebuilds tables, and with enforcement on a rebuild would delete
// the child rows of the table it replaces. migrate receives the pinned connection and must use only it.
//
// Precondition: db must be the root *gorm.DB, never a transaction handle. db.Connection pins a pool connection, so
// the pause, migration and repair run on that connection and outside any transaction opened on db. Passing a
// transaction would not make them run inside it, and the pause would then fail (enforcement cannot change inside a
// transaction).
//
// Only a failure to pause enforcement stops the migration. Repair and check problems are logged and do not fail it.
func MigrateWithIntegrity(db *gorm.DB, migrate func(conn *gorm.DB) error) error {
	return db.Connection(func(conn *gorm.DB) error {
		if err := PauseForeignKeys(conn); err != nil {
			return fmt.Errorf("pause reference enforcement: %w", err)
		}

		migrateErr := migrate(conn)
		if migrateErr == nil {
			results, err := RepairReferences(conn)
			if err != nil {
				logger.Log().WithError(err).Error("Repairing database references failed; the repair changes were rolled back")
			}
			ReportReferenceCheck(conn, results)
		}
		if err := ResumeForeignKeys(conn); err != nil {
			logger.Log().WithError(err).Error("Could not turn database reference checks back on; the connection is replaced")
		}
		return migrateErr
	})
}

// PauseForeignKeys switches reference enforcement off for conn and checks that it is off. Enforcement cannot be
// changed inside a transaction, so the check is what catches that case.
func PauseForeignKeys(conn *gorm.DB) error {
	return setForeignKeys(conn, false)
}

// ResumeForeignKeys switches reference enforcement on for conn. If that fails, conn is removed from the pool, so the
// next query runs on a connection that gets enforcement from the connection string.
func ResumeForeignKeys(conn *gorm.DB) error {
	if err := setForeignKeys(conn, true); err != nil {
		discardPinnedConnection(conn)
		return err
	}
	return nil
}

// setForeignKeys switches reference enforcement on or off for conn and reads the setting back. It returns
// errForeignKeysStillOn when turning it off does not take effect.
func setForeignKeys(conn *gorm.DB, on bool) error {
	value, want := "OFF", 0
	if on {
		value, want = "ON", 1
	}
	if err := conn.Exec("PRAGMA foreign_keys = " + value).Error; err != nil {
		return fmt.Errorf("set reference enforcement %s: %w", value, err)
	}
	var got int
	if err := conn.Raw("PRAGMA foreign_keys").Scan(&got).Error; err != nil {
		return fmt.Errorf("read reference enforcement: %w", err)
	}
	if got != want {
		if !on {
			return errForeignKeysStillOn
		}
		return fmt.Errorf("reference enforcement reads %d after setting %s", got, value)
	}
	return nil
}

// discardPinnedConnection removes the pinned connection from the pool so the next query opens a fresh one.
func discardPinnedConnection(conn *gorm.DB) {
	if sc, ok := conn.Statement.ConnPool.(*sql.Conn); ok {
		_ = sc.Raw(func(any) error { return driver.ErrBadConn })
	}
}

// RepairReferences clears or deletes every stored row that breaks a declared reference, in one transaction. It can
// run inside a caller's transaction (a savepoint is used), so a caller can roll everything back on error. Changes
// are logged once the transaction commits. Rules whose tables or columns are missing are skipped.
func RepairReferences(tx *gorm.DB) ([]RepairResult, error) {
	var results []RepairResult
	err := tx.Transaction(func(inner *gorm.DB) error {
		results = nil
		for _, rule := range ReferenceRules {
			result, changed, err := repairRule(inner, rule)
			if err != nil {
				return err
			}
			if changed {
				results = append(results, result)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("repair database references: %w", err)
	}
	logRepairs(results)
	return results, nil
}

// repairRule applies one rule and reports whether any row changed.
func repairRule(tx *gorm.DB, rule ReferenceRule) (RepairResult, bool, error) {
	migrator := tx.Migrator()
	if !migrator.HasTable(rule.Child) || !migrator.HasColumn(rule.Child, rule.Column) || !migrator.HasTable(rule.Parent) {
		logger.Log().WithField("table", rule.Child).WithField("column", rule.Column).Debug("Skipped a reference rule whose table is not in the schema")
		return RepairResult{}, false, nil
	}

	var ids []any
	if err := brokenRows(tx, rule).Pluck("id", &ids).Error; err != nil {
		return RepairResult{}, false, fmt.Errorf("find broken %s.%s: %w", rule.Child, rule.Column, err)
	}
	if len(ids) == 0 {
		return RepairResult{}, false, nil
	}
	sample := make([]string, 0, maxLoggedIDs)
	for _, id := range ids {
		if len(sample) == maxLoggedIDs {
			break
		}
		sample = append(sample, fmt.Sprint(id))
	}
	result := RepairResult{Rule: rule, Count: len(ids), SampleIDs: sample}

	switch rule.Action {
	case RepairClear:
		updates := map[string]any{rule.Column: nil}
		for column, value := range rule.AlsoSet {
			updates[column] = value
		}
		if err := tx.Table(rule.Child).Where(idIn(rule.Child, ids)).Updates(updates).Error; err != nil {
			return RepairResult{}, false, fmt.Errorf("clear %s.%s: %w", rule.Child, rule.Column, err)
		}
	case RepairDeleteRow:
		for _, dependent := range rule.AlsoDelete {
			dependentIn := clause.IN{Column: clause.Column{Table: dependent.Table, Name: dependent.Column}, Values: ids}
			if err := tx.Table(dependent.Table).Where(dependentIn).Delete(map[string]any{}).Error; err != nil {
				return RepairResult{}, false, fmt.Errorf("delete %s rows of removed %s: %w", dependent.Table, rule.Child, err)
			}
		}
		if err := tx.Table(rule.Child).Where(idIn(rule.Child, ids)).Delete(map[string]any{}).Error; err != nil {
			return RepairResult{}, false, fmt.Errorf("delete %s rows: %w", rule.Child, err)
		}
	default:
		return RepairResult{}, false, fmt.Errorf("unknown repair action %d for %s.%s", rule.Action, rule.Child, rule.Column)
	}
	return result, true, nil
}

// brokenRows selects the child rows of rule whose reference is set and whose parent row does not exist.
func brokenRows(tx *gorm.DB, rule ReferenceRule) *gorm.DB {
	child := clause.Column{Table: rule.Child, Name: rule.Column}
	parent := tx.Session(&gorm.Session{NewDB: true}).Table(rule.Parent).Select("1").
		Where(clause.Eq{Column: clause.Column{Table: rule.Parent, Name: rule.ParentColumn}, Value: child})
	return tx.Table(rule.Child).
		Where(clause.Expr{SQL: "? IS NOT NULL", Vars: []any{child}}).
		Where(clause.Expr{SQL: "NOT EXISTS (?)", Vars: []any{parent}})
}

// idIn matches rows of table whose id is one of ids.
func idIn(table string, ids []any) clause.IN {
	return clause.IN{Column: clause.Column{Table: table, Name: "id"}, Values: ids}
}

// logRepairs writes one Warn per rule that changed rows. Only ids and counts are logged.
func logRepairs(results []RepairResult) {
	for _, result := range results {
		entry := logger.Log().
			WithField("table", result.Rule.Child).
			WithField("column", result.Rule.Column).
			WithField("action", result.Rule.Action.String()).
			WithField("count", result.Count).
			WithField("ids", strings.Join(result.SampleIDs, ","))
		if len(result.Rule.AlsoSet) > 0 {
			entry = entry.WithField("dns_challenge_disabled", result.Count)
		}
		entry.Warn("Repaired database references")
	}
}

// CheckReferences lists the rows that still break a declared reference (PRAGMA foreign_key_check). It works with
// enforcement on or off.
func CheckReferences(conn *gorm.DB) ([]ReferenceViolation, error) {
	rows, err := conn.Raw("PRAGMA foreign_key_check").Rows()
	if err != nil {
		return nil, fmt.Errorf("run foreign_key_check: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var violations []ReferenceViolation
	for rows.Next() {
		var v ReferenceViolation
		var rowID sql.NullInt64
		if err := rows.Scan(&v.Table, &rowID, &v.Parent, &v.FKID); err != nil {
			return nil, fmt.Errorf("read foreign_key_check row: %w", err)
		}
		v.RowID = rowID.Int64
		violations = append(violations, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read foreign_key_check rows: %w", err)
	}
	return violations, nil
}

// ReportReferenceCheck runs the check on conn and logs the summary: an Info line when anything was repaired or
// remains broken, Debug otherwise, and one Warn per child table that still holds broken rows.
func ReportReferenceCheck(conn *gorm.DB, results []RepairResult) {
	violations, err := CheckReferences(conn)
	if err != nil {
		logger.Log().WithError(err).Warn("Database reference check could not run")
		return
	}
	repaired := 0
	for _, result := range results {
		repaired += result.Count
	}
	entry := logger.Log().WithField("repaired", repaired).WithField("remaining", len(violations))
	if repaired == 0 && len(violations) == 0 {
		entry.Debug("Database reference check finished")
	} else {
		entry.Info("Database reference check finished")
	}
	logRemainingViolations(violations)
}

// logRemainingViolations writes one Warn per child table that still holds broken references.
func logRemainingViolations(violations []ReferenceViolation) {
	counts := map[string]int{}
	var order []string
	for _, v := range violations {
		if counts[v.Table] == 0 {
			order = append(order, v.Table)
		}
		counts[v.Table]++
	}
	for _, table := range order {
		logger.Log().WithField("table", table).WithField("count", counts[table]).Warn(
			"Charon keeps running; saving one of these rows can fail until the referenced item is restored or the row is edited")
	}
}

// ClearMissingReferences clears, in row, every reference of table whose parent row no longer exists, and applies the
// AlsoSet values of the cleared rules. row maps column names to values (a pointer to an id, or nil); it is changed in
// place. It returns the cleared columns. Callers copy the values back into their struct.
func ClearMissingReferences(tx *gorm.DB, table string, row map[string]any) ([]string, error) {
	var cleared []string
	for _, rule := range ReferenceRules {
		if rule.Child != table || rule.Action != RepairClear {
			continue
		}
		value, set := referenceValue(row[rule.Column])
		if !set {
			continue
		}
		exists, err := parentExists(tx, rule, value)
		if err != nil {
			return cleared, err
		}
		if exists {
			continue
		}
		row[rule.Column] = nil
		for column, v := range rule.AlsoSet {
			row[column] = v
		}
		cleared = append(cleared, rule.Column)
	}
	return cleared, nil
}

// referenceValue returns the id held by v (a pointer or a plain value) and whether one is set.
func referenceValue(v any) (any, bool) {
	if v == nil {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false
		}
		return rv.Elem().Interface(), true
	}
	return v, true
}

// parentExists reports whether the parent row of rule holds value.
func parentExists(tx *gorm.DB, rule ReferenceRule, value any) (bool, error) {
	var count int64
	err := tx.Table(rule.Parent).
		Where(clause.Eq{Column: clause.Column{Table: rule.Parent, Name: rule.ParentColumn}, Value: value}).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check %s.%s: %w", rule.Parent, rule.ParentColumn, err)
	}
	return count > 0, nil
}
