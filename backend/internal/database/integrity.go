package database

import (
	"fmt"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Table names of the 12 declared references (see ReferenceRules).
const (
	TableBackupRecords          = "backup_records"
	TableBackupRemoteCopies     = "backup_remote_copies"
	TableDNSProviders           = "dns_providers"
	TableDNSProviderCredentials = "dns_provider_credentials"
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
	{Child: TableDNSProviderCredentials, Column: "dns_provider_id", Parent: TableDNSProviders, ParentColumn: "id", Action: RepairDeleteRow},
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
