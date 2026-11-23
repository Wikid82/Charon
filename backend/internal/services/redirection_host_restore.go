package services

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Wikid82/charon/backend/internal/models"
)

// readRedirectionHost reads the stored redirection host with id through tx, without preloads.
func readRedirectionHost(tx *gorm.DB, id uint) (models.RedirectionHost, error) {
	var host models.RedirectionHost
	if err := tx.Where("id = ?", id).First(&host).Error; err != nil {
		return models.RedirectionHost{}, fmt.Errorf("read redirection host %d: %w", id, err)
	}
	return host, nil
}

// storedRedirectionState returns the comparable stored state of a redirection host, timestamps cleared.
func storedRedirectionState(host models.RedirectionHost) models.RedirectionHost {
	host.CreatedAt, host.UpdatedAt = time.Time{}, time.Time{}
	return host
}

// Snapshot returns the stored state of a redirection host.
func (s *RedirectionHostService) Snapshot(id uint) (models.RedirectionHost, error) {
	return readRedirectionHost(s.db, id)
}

// CreateAsSnapshot is CreateAs that also returns the stored state the create left, read in the same write-locked
// transaction (see ProxyHostService.CreateAsSnapshot).
func (s *RedirectionHostService) CreateAsSnapshot(host *models.RedirectionHost, admin bool) (models.RedirectionHost, error) {
	var written models.RedirectionHost
	err := WithWriteLock(s.db, &models.RedirectionHost{}, func(tx *gorm.DB) error {
		if err := s.checkHostWrite(tx, host, 0, admin); err != nil {
			return err
		}
		if err := tx.Create(host).Error; err != nil {
			return err
		}
		var readErr error
		written, readErr = readRedirectionHost(tx, host.ID)
		return readErr
	})
	return written, err
}

// UpdateAsSnapshot is UpdateAs that also returns the stored state the update replaced (before) and the state it left
// (written), both read in the same write-locked transaction as the write.
func (s *RedirectionHostService) UpdateAsSnapshot(host *models.RedirectionHost, admin bool) (before, written models.RedirectionHost, err error) {
	err = WithWriteLock(s.db, &models.RedirectionHost{}, func(tx *gorm.DB) error {
		if err := s.checkHostWrite(tx, host, host.ID, admin); err != nil {
			return err
		}
		snap, err := readRedirectionHost(tx, host.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // a row that is gone matches nothing, as UpdateAs always did
		}
		if err != nil {
			return err
		}
		before = snap
		// Use Updates+Select("*") to handle nullable foreign keys properly,
		// mirroring ProxyHostService.Update.
		if err := tx.Model(&models.RedirectionHost{}).
			Where("id = ?", host.ID).
			Select("*").
			Updates(host).Error; err != nil {
			return err
		}
		written, err = readRedirectionHost(tx, host.ID)
		return err
	})
	if err != nil {
		return models.RedirectionHost{}, models.RedirectionHost{}, err
	}
	return before, written, nil
}

// RestoreUpdated puts snap back while the stored row still holds what the refused update wrote (written).
// Otherwise it keeps the newer state and returns ErrRestoreSuperseded.
func (s *RedirectionHostService) RestoreUpdated(snap, written models.RedirectionHost) error {
	return WithWriteLock(s.db, &models.RedirectionHost{}, func(tx *gorm.DB) error {
		current, err := readRedirectionHost(tx, snap.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(storedRedirectionState(current), storedRedirectionState(written)) {
			return ErrRestoreSuperseded
		}
		return tx.Model(&models.RedirectionHost{}).
			Where("id = ?", snap.ID).
			Select("*").
			Omit(clause.Associations).
			Updates(&snap).Error
	})
}

// RemoveCreated deletes the host while it still holds what the refused create wrote (written).
func (s *RedirectionHostService) RemoveCreated(written models.RedirectionHost) error {
	return WithWriteLock(s.db, &models.RedirectionHost{}, func(tx *gorm.DB) error {
		current, err := readRedirectionHost(tx, written.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRestoreSuperseded
		}
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(storedRedirectionState(current), storedRedirectionState(written)) {
			return ErrRestoreSuperseded
		}
		return tx.Delete(&models.RedirectionHost{}, written.ID).Error
	})
}

// ReinsertDeleted puts a deleted redirection host back with its original id. It is skipped, with
// ErrRestoreSuperseded, when a host with the same id or UUID exists or a domain of the host is now claimed.
func (s *RedirectionHostService) ReinsertDeleted(snap models.RedirectionHost) error {
	return WithWriteLock(s.db, &models.RedirectionHost{}, func(tx *gorm.DB) error {
		var taken int64
		if err := tx.Model(&models.RedirectionHost{}).
			Where("id = ? OR uuid = ?", snap.ID, snap.UUID).
			Count(&taken).Error; err != nil {
			return fmt.Errorf("check redirection host %d: %w", snap.ID, err)
		}
		if taken > 0 {
			return fmt.Errorf("%w: the host exists again", ErrRestoreSuperseded)
		}
		if err := checkSameTableDomainConflict(tx, snap.DomainNames, &models.RedirectionHost{}, 0); err != nil {
			return fmt.Errorf("%w: %w", ErrRestoreSuperseded, err)
		}
		if err := checkCrossTableDomainConflict(tx, snap.DomainNames, &models.ProxyHost{}); err != nil {
			return fmt.Errorf("%w: %w", ErrRestoreSuperseded, err)
		}
		if err := insertKeepingValues(tx, &snap); err != nil {
			return fmt.Errorf("re-insert redirection host %d: %w", snap.ID, err)
		}
		return nil
	})
}
