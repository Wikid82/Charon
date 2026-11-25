package services

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// ErrRestoreSuperseded reports that a restore was skipped because the stored row no longer holds the value the
// refused request wrote: a newer change was kept.
var ErrRestoreSuperseded = errors.New("a newer change was kept")

// ProxyHostSnapshot is a proxy host and its location rows as stored at one point. Host carries no preloaded
// associations.
type ProxyHostSnapshot struct {
	Host      models.ProxyHost
	Locations []models.Location
}

// readProxyHostSnapshot reads the stored row and its locations through tx.
func readProxyHostSnapshot(tx *gorm.DB, id uint) (ProxyHostSnapshot, error) {
	var host models.ProxyHost
	if err := tx.Where("id = ?", id).First(&host).Error; err != nil {
		return ProxyHostSnapshot{}, fmt.Errorf("read proxy host %d: %w", id, err)
	}
	var locations []models.Location
	if err := tx.Where("proxy_host_id = ?", id).Order("id").Find(&locations).Error; err != nil {
		return ProxyHostSnapshot{}, fmt.Errorf("read locations of proxy host %d: %w", id, err)
	}
	return ProxyHostSnapshot{Host: host, Locations: locations}, nil
}

// Snapshot returns the stored state of a proxy host and its locations.
func (s *ProxyHostService) Snapshot(id uint) (ProxyHostSnapshot, error) {
	return readProxyHostSnapshot(s.db, id)
}

// storedStateOf returns the comparable stored state of a snapshot: the persisted columns and the location rows,
// with every timestamp cleared. Two snapshots with the same stored state hold the same configuration.
func storedStateOf(snap ProxyHostSnapshot) ProxyHostSnapshot {
	host := snap.Host
	host.CreatedAt, host.UpdatedAt = time.Time{}, time.Time{}
	locations := make([]models.Location, len(snap.Locations))
	for i, loc := range snap.Locations {
		loc.CreatedAt, loc.UpdatedAt = time.Time{}, time.Time{}
		locations[i] = loc
	}
	return ProxyHostSnapshot{Host: host, Locations: locations}
}

// sameStoredState reports whether two snapshots hold the same stored configuration (timestamps excluded).
func sameStoredState(a, b ProxyHostSnapshot) bool {
	return reflect.DeepEqual(storedStateOf(a), storedStateOf(b))
}

// writeProxyHostSnapshot replaces the stored row and its locations with snap, keeping the original ids.
func writeProxyHostSnapshot(tx *gorm.DB, snap ProxyHostSnapshot) error {
	id := snap.Host.ID
	if err := tx.Model(&models.ProxyHost{}).
		Where("id = ?", id).
		Select("*").
		Omit(clause.Associations).
		Updates(&snap.Host).Error; err != nil {
		return fmt.Errorf("restore proxy host %d: %w", id, err)
	}
	if err := tx.Where("proxy_host_id = ?", id).Delete(&models.Location{}).Error; err != nil {
		return fmt.Errorf("clear locations of proxy host %d: %w", id, err)
	}
	return insertProxyHostLocations(tx, snap.Locations)
}

// insertKeepingValues inserts row (a pointer to a struct) with its original ids and then writes every column back with
// the value row held before the insert. GORM replaces the zero value of a column with a default tag by that default on
// insert, both in the SQL and in the struct, so a stored false or "" would otherwise come back as the default. The
// values are copied before the insert. Associations are not written.
func insertKeepingValues(tx *gorm.DB, row any) error {
	want := reflect.New(reflect.TypeOf(row).Elem())
	want.Elem().Set(reflect.ValueOf(row).Elem())
	if err := tx.Omit(clause.Associations).Create(row).Error; err != nil {
		return err
	}
	return tx.Model(row).Select("*").Omit(clause.Associations).Updates(want.Interface()).Error
}

// insertProxyHostLocations inserts location rows with their original ids and stored values.
func insertProxyHostLocations(tx *gorm.DB, locations []models.Location) error {
	for i := range locations {
		row := locations[i]
		if err := insertKeepingValues(tx, &row); err != nil {
			return fmt.Errorf("insert location %d of proxy host %d: %w", row.ID, row.ProxyHostID, err)
		}
	}
	return nil
}

// RestoreUpdated puts snap back while the stored row and its locations still hold what the refused write stored
// (written). Otherwise it keeps the newer state and returns ErrRestoreSuperseded.
func (s *ProxyHostService) RestoreUpdated(snap, written ProxyHostSnapshot) error {
	err := WithWriteLock(s.db, &models.ProxyHost{}, func(tx *gorm.DB) error {
		current, err := readProxyHostSnapshot(tx, snap.Host.ID)
		if err != nil {
			return err
		}
		if !sameStoredState(current, written) {
			return ErrRestoreSuperseded
		}
		return writeProxyHostSnapshot(tx, snap)
	})
	if err != nil {
		return err
	}
	s.invalidateCertCache()
	return nil
}

// RemoveCreated deletes the host and its locations while they still hold what the refused create stored
// (written). A host edited since is kept and ErrRestoreSuperseded is returned.
func (s *ProxyHostService) RemoveCreated(written ProxyHostSnapshot) error {
	err := WithWriteLock(s.db, &models.ProxyHost{}, func(tx *gorm.DB) error {
		current, err := readProxyHostSnapshot(tx, written.Host.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRestoreSuperseded
		}
		if err != nil {
			return err
		}
		if !sameStoredState(current, written) {
			return ErrRestoreSuperseded
		}
		if err := tx.Where("proxy_host_id = ?", written.Host.ID).Delete(&models.Location{}).Error; err != nil {
			return fmt.Errorf("remove locations of proxy host %d: %w", written.Host.ID, err)
		}
		// The periodic check may have created a monitor for the host after the create; it goes with the host.
		if err := deleteHostMonitors(tx, written.Host.ID); err != nil {
			return err
		}
		if err := tx.Delete(&models.ProxyHost{}, written.Host.ID).Error; err != nil {
			return fmt.Errorf("remove proxy host %d: %w", written.Host.ID, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.invalidateCertCache()
	return nil
}

// ReinsertDeleted puts a deleted host back with its original ids and locations. It is skipped, with
// ErrRestoreSuperseded, when a host with the same UUID exists or a domain of the host is now claimed.
func (s *ProxyHostService) ReinsertDeleted(snap ProxyHostSnapshot) error {
	err := WithWriteLock(s.db, &models.ProxyHost{}, func(tx *gorm.DB) error {
		var taken int64
		if err := tx.Model(&models.ProxyHost{}).
			Where("id = ? OR uuid = ?", snap.Host.ID, snap.Host.UUID).
			Count(&taken).Error; err != nil {
			return fmt.Errorf("check proxy host %d: %w", snap.Host.ID, err)
		}
		if taken > 0 {
			return fmt.Errorf("%w: the host exists again", ErrRestoreSuperseded)
		}
		if err := checkSameTableDomainConflict(tx, snap.Host.DomainNames, &models.ProxyHost{}, 0); err != nil {
			return fmt.Errorf("%w: %w", ErrRestoreSuperseded, err)
		}
		if err := checkCrossTableDomainConflict(tx, snap.Host.DomainNames, &models.RedirectionHost{}); err != nil {
			return fmt.Errorf("%w: %w", ErrRestoreSuperseded, err)
		}
		host := snap.Host
		host.Locations = nil
		if err := clearMissingProxyHostReferences(tx, &host); err != nil {
			return fmt.Errorf("clear references of proxy host %d: %w", snap.Host.ID, err)
		}
		// A delete leaves the location rows behind; they are replaced by the snapshot copies.
		if err := tx.Where("proxy_host_id = ?", snap.Host.ID).Delete(&models.Location{}).Error; err != nil {
			return fmt.Errorf("clear locations of proxy host %d: %w", snap.Host.ID, err)
		}
		if err := insertKeepingValues(tx, &host); err != nil {
			return fmt.Errorf("re-insert proxy host %d: %w", snap.Host.ID, err)
		}
		return insertProxyHostLocations(tx, snap.Locations)
	})
	if err != nil {
		return err
	}
	s.invalidateCertCache()
	return nil
}

// ColumnChange is one bulk-updated host: the value the column held before the bulk write and the updated_at the
// write left on the row.
type ColumnChange struct {
	ID        uint
	Old       *uint
	WrittenAt time.Time
}

// restorableHostColumns is the allow-list of columns RestoreColumn may write.
var restorableHostColumns = map[string]bool{
	"access_list_id":             true,
	"proxy_group_id":             true,
	"security_header_profile_id": true,
}

// RestoreColumn sets column back to the old value for every change whose row still carries the updated_at the
// bulk write left. Rows changed since are kept; when any is kept the error wraps ErrRestoreSuperseded, after the
// others are restored.
func (s *ProxyHostService) RestoreColumn(column string, changes []ColumnChange) error {
	if !restorableHostColumns[column] {
		return fmt.Errorf("column %q cannot be restored", column)
	}
	kept := 0
	err := WithWriteLock(s.db, &models.ProxyHost{}, func(tx *gorm.DB) error {
		for _, change := range changes {
			var current models.ProxyHost
			err := tx.Select("id", "updated_at").Where("id = ?", change.ID).First(&current).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				kept++
				continue
			}
			if err != nil {
				return fmt.Errorf("read proxy host %d: %w", change.ID, err)
			}
			if !current.UpdatedAt.Equal(change.WrittenAt) {
				kept++
				continue
			}
			if err := tx.Model(&models.ProxyHost{}).
				Where("id = ?", change.ID).
				Updates(map[string]any{column: change.Old, "updated_at": time.Now()}).Error; err != nil {
				return fmt.Errorf("restore %s of proxy host %d: %w", column, change.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.invalidateCertCache()
	if kept > 0 {
		return fmt.Errorf("%d host(s) changed since the write: %w", kept, ErrRestoreSuperseded)
	}
	return nil
}

// deleteHostMonitors removes the uptime monitors of a proxy host and their heartbeats. A monitor row points at its
// host, so it must be gone before the host row is deleted.
func deleteHostMonitors(tx *gorm.DB, hostID uint) error {
	if !tx.Migrator().HasTable(&models.UptimeMonitor{}) {
		return nil
	}
	var monitorIDs []string
	if err := tx.Model(&models.UptimeMonitor{}).Where("proxy_host_id = ?", hostID).Pluck("id", &monitorIDs).Error; err != nil {
		return fmt.Errorf("list monitors of proxy host %d: %w", hostID, err)
	}
	if len(monitorIDs) == 0 {
		return nil
	}
	if err := tx.Where("monitor_id IN ?", monitorIDs).Delete(&models.UptimeHeartbeat{}).Error; err != nil {
		return fmt.Errorf("delete heartbeats of proxy host %d: %w", hostID, err)
	}
	if err := tx.Where("id IN ?", monitorIDs).Delete(&models.UptimeMonitor{}).Error; err != nil {
		return fmt.Errorf("delete monitors of proxy host %d: %w", hostID, err)
	}
	return nil
}

// clearMissingProxyHostReferences clears the references of host whose parent row is gone, so the restored host can be
// saved. Each cleared reference is logged by column name and host id only.
func clearMissingProxyHostReferences(tx *gorm.DB, host *models.ProxyHost) error {
	refs := map[string]any{
		"certificate_id":             host.CertificateID,
		"access_list_id":             host.AccessListID,
		"security_header_profile_id": host.SecurityHeaderProfileID,
		"dns_provider_id":            host.DNSProviderID,
		"proxy_group_id":             host.ProxyGroupID,
		"use_dns_challenge":          host.UseDNSChallenge,
	}
	cleared, err := database.ClearMissingReferences(tx, database.TableProxyHosts, refs)
	if err != nil {
		return err
	}
	if len(cleared) == 0 {
		return nil
	}
	host.CertificateID = optionalID(refs["certificate_id"])
	host.AccessListID = optionalID(refs["access_list_id"])
	host.SecurityHeaderProfileID = optionalID(refs["security_header_profile_id"])
	host.DNSProviderID = optionalID(refs["dns_provider_id"])
	host.ProxyGroupID = optionalID(refs["proxy_group_id"])
	host.UseDNSChallenge, _ = refs["use_dns_challenge"].(bool)
	logClearedReferences(host.UUID, cleared)
	return nil
}

// optionalID returns the id held by v, or nil when v is not a set id.
func optionalID(v any) *uint {
	id, _ := v.(*uint)
	return id
}

// logClearedReferences writes one Warn per cleared column. Only the column names and the item UUID are logged.
func logClearedReferences(hostUUID string, cleared []string) {
	for _, column := range cleared {
		entry := logger.Log().WithField("host_uuid", hostUUID).WithField("column", column)
		if column == "dns_provider_id" {
			entry.Warn("DNS challenge turned off because its DNS provider no longer exists")
			continue
		}
		entry.Warn("Cleared a reference to an item that no longer exists before restoring the host")
	}
}
