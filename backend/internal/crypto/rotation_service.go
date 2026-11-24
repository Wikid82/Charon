// Package crypto provides cryptographic services for sensitive data.
package crypto

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// Sentinel errors returned by the key service.
var (
	// ErrNoEncryptionKey means no usable key is configured. A nil *RotationService
	// reports it, so a consumer handed a typed nil treats it as "no key".
	ErrNoEncryptionKey = errors.New("no encryption key is configured")
	// ErrNoLoadedKeyOpens means no configured key authenticates the ciphertext.
	ErrNoLoadedKeyOpens = errors.New("no configured encryption key opens this value")
	// ErrRotationNotNeeded means every stored secret is already on the only target
	// key (the current key) and no next key is configured.
	// ErrKeyRegistryUnavailable means the keys were parsed but the key registry in
	// the database could not be read or written when the rotation service was built.
	ErrKeyRegistryUnavailable = errors.New("encryption key registry unavailable")
	ErrRotationNotNeeded      = errors.New("nothing to re-encrypt: every stored secret already uses the current key; set CHARON_ENCRYPTION_KEY_NEXT to move to a new key")
)

// Environment variable names of the key slots, used in logs and status.
const (
	currentSlotName = "CHARON_ENCRYPTION_KEY"
	nextSlotName    = "CHARON_ENCRYPTION_KEY_NEXT"
	legacySlotFmt   = "CHARON_ENCRYPTION_KEY_V%d"
	// maxLegacySlot is the highest CHARON_ENCRYPTION_KEY_V<n> slot.
	maxLegacySlot = 10
	// firstLegacyVersion is the first version given to a legacy key when the
	// registry is filled for the first time. 1 is the current key and 2 is
	// reserved for the next key, as older releases tagged them.
	firstLegacyVersion = 3
)

// keyIDLabel is the message of the HMAC that derives a key's registry id.
const keyIDLabel = "charon/encryption-key-id/v1"

// keyIDLength is the number of hex characters kept from the HMAC output.
const keyIDLength = 32

// keyRole says what a configured slot is used for.
type keyRole int

const (
	roleCurrent keyRole = iota
	roleNext
	roleLegacy
)

// keySlot is one configured key variable. svc is nil when the value is set but
// could not be loaded (the variable name is logged, never the value).
type keySlot struct {
	name  string
	role  keyRole
	svc   *EncryptionService
	keyID string
}

// keyState is the version map used for every read and write. It is immutable:
// Rehydrate builds a new one and swaps it in under the service's lock.
type keyState struct {
	byVersion      map[int]*EncryptionService
	versions       map[string]int // key id -> version
	currentVersion int
	nextVersion    int // 0 when no next key is loaded
}

// RotationService is the one key service shared by every encrypted store. It
// holds the configured keys, gives each a stable registry version, decrypts with
// the version a row is tagged with (falling back to every loaded key), and
// re-encrypts stored secrets during rotation.
type RotationService struct {
	db    *gorm.DB
	slots []keySlot // immutable after construction

	mu    sync.RWMutex
	state *keyState
}

// StoreRotation is the outcome of rotation for one encrypted store.
type StoreRotation struct {
	Store          string `json:"store"`
	Total          int    `json:"total"`
	Rotated        int    `json:"rotated"`
	AlreadyCurrent int    `json:"already_current"`
	Unreadable     int    `json:"unreadable"`
}

// RotationResult is the outcome of one RotateAll run.
type RotationResult struct {
	TargetVersion       int             `json:"target_version"`
	Stores              []StoreRotation `json:"stores"`
	TotalRecords        int             `json:"total_records"`
	RotatedCount        int             `json:"rotated_count"`
	AlreadyCurrentCount int             `json:"already_current_count"`
	UnreadableCount     int             `json:"unreadable_count"`
	NewKeyVersion       int             `json:"new_key_version"`
	Duration            string          `json:"duration"`
	StartedAt           time.Time       `json:"started_at"`
	CompletedAt         time.Time       `json:"completed_at"`
}

// KeyUsage describes one configured key slot.
type KeyUsage struct {
	Slot      string `json:"slot"`
	Version   int    `json:"version"`
	KeyID     string `json:"key_id"`
	InUse     int    `json:"in_use"`
	Removable bool   `json:"removable"`
	Loaded    bool   `json:"loaded"`
}

// StoreUsage counts the rows of one encrypted store by the key that opens them.
type StoreUsage struct {
	Store      string      `json:"store"`
	Total      int         `json:"total"`
	ByVersion  map[int]int `json:"by_version"`
	Unreadable int         `json:"unreadable"`
}

// RotationStatus describes the configured keys and where the stored secrets are.
// Usage is verified by decryption, not by the version tags.
type RotationStatus struct {
	CurrentVersion    int          `json:"current_version"`
	NextKeyConfigured bool         `json:"next_key_configured"`
	NextVersion       int          `json:"next_version"`
	LegacyKeyCount    int          `json:"legacy_key_count"`
	LegacyKeyVersions []int        `json:"legacy_key_versions"`
	Keys              []KeyUsage   `json:"keys"`
	Stores            []StoreUsage `json:"stores"`
	UnreadableTotal   int          `json:"unreadable_total"`
	RotationNeeded    bool         `json:"rotation_needed"`

	// DNS providers only; kept for scripts that read the earlier fields.
	ProvidersOnCurrentVersion int         `json:"providers_on_current_version"`
	ProvidersOnOlderVersions  int         `json:"providers_on_older_versions"`
	ProvidersByVersion        map[int]int `json:"providers_by_version"`
}

// KeyValidationError lists the slots whose key failed validation. It carries
// slot names only, never key material.
type KeyValidationError struct {
	Slots []string
}

// Error describes the failing slots.
func (e *KeyValidationError) Error() string {
	return "encryption key validation failed for: " + strings.Join(e.Slots, ", ")
}

// usageReport is the per-row scan that status and reconcile share.
type usageReport struct {
	stores     []StoreUsage
	inUse      map[int]int // readable rows per version of the key that opens them
	unreadable []string    // "store#id" for each row no key opens
	misplaced  int         // readable rows not opened by the rotation target
}

// NewRotationService loads the configured keys and registers them in the key
// registry. The current key is required. A next or legacy key that cannot be
// loaded is logged by variable name and skipped, and shows as not loaded.
func NewRotationService(db *gorm.DB, keys config.EncryptionKeys) (*RotationService, error) {
	slots, err := loadKeySlots(keys)
	if err != nil {
		return nil, err
	}
	rs := &RotationService{db: db, slots: slots}
	if err := rs.reloadKeys(context.Background()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKeyRegistryUnavailable, err)
	}
	return rs, nil
}

// loadKeySlots decodes every configured key. Duplicates of the current key are
// dropped (a next key equal to the current key is not configured).
func loadKeySlots(keys config.EncryptionKeys) ([]keySlot, error) {
	if keys.Current == "" {
		return nil, fmt.Errorf("%s is required", currentSlotName)
	}
	current, err := NewEncryptionService(keys.Current)
	if err != nil {
		return nil, fmt.Errorf("failed to load current encryption key: %w", err)
	}
	slots := []keySlot{{name: currentSlotName, role: roleCurrent, svc: current, keyID: keyIDFor(current)}}

	if keys.Next != "" {
		next := loadOptionalSlot(nextSlotName, roleNext, keys.Next)
		if next.svc != nil && next.keyID == slots[0].keyID {
			logger.Log().WithField("slot", nextSlotName).Info("Encryption key duplicates the current key and is ignored")
		} else {
			slots = append(slots, next)
		}
	}

	for slot := 1; slot <= maxLegacySlot; slot++ {
		value := keys.Legacy[slot]
		if value == "" {
			continue
		}
		slots = append(slots, loadOptionalSlot(fmt.Sprintf(legacySlotFmt, slot), roleLegacy, value))
	}
	return slots, nil
}

// loadOptionalSlot decodes a next or legacy key. A key that fails to load is
// returned with a nil service; the warning names the variable only.
func loadOptionalSlot(name string, role keyRole, keyB64 string) keySlot {
	svc, err := NewEncryptionService(keyB64)
	if err != nil {
		logger.Log().WithError(err).WithField("slot", name).Warn("Encryption key could not be loaded and is skipped")
		return keySlot{name: name, role: role}
	}
	return keySlot{name: name, role: role, svc: svc, keyID: keyIDFor(svc)}
}

// keyIDFor derives the registry id of a key from its decoded bytes.
func keyIDFor(svc *EncryptionService) string {
	mac := hmac.New(sha256.New, svc.key)
	mac.Write([]byte(keyIDLabel))
	return hex.EncodeToString(mac.Sum(nil))[:keyIDLength]
}

// reloadKeys registers the loaded keys (inside the database write lock) and swaps
// in the resulting version map.
func (rs *RotationService) reloadKeys(ctx context.Context) error {
	var versions map[string]int
	err := database.WithWriteLock(rs.db.WithContext(ctx), &models.EncryptionKeyVersion{}, func(tx *gorm.DB) error {
		var registerErr error
		versions, registerErr = registerKeys(tx, rs.slots, time.Now())
		return registerErr
	})
	if err != nil {
		return fmt.Errorf("register encryption keys: %w", err)
	}

	state := &keyState{byVersion: map[int]*EncryptionService{}, versions: versions}
	for _, slot := range rs.slots {
		if slot.svc == nil {
			continue
		}
		version := versions[slot.keyID]
		state.byVersion[version] = slot.svc
		switch slot.role {
		case roleCurrent:
			state.currentVersion = version
		case roleNext:
			state.nextVersion = version
		case roleLegacy:
		}
	}

	rs.mu.Lock()
	rs.state = state
	rs.mu.Unlock()
	return nil
}

// registerKeys gives every loaded key a version and returns key id -> version.
// A key already in the registry keeps its version. A new key takes max+1, except
// when the registry is empty (an upgrade, or a restore of a backup without the
// table): then current is 1, next is 2 and legacy keys take 3, 4, ... in slot
// order. Last-seen times are updated for every loaded key.
func registerKeys(tx *gorm.DB, slots []keySlot, now time.Time) (map[string]int, error) {
	var rows []models.EncryptionKeyVersion
	if err := tx.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("read key registry: %w", err)
	}
	byKeyID := make(map[string]int, len(rows))
	maxVersion := 0
	for _, row := range rows {
		byKeyID[row.KeyID] = row.Version
		if row.Version > maxVersion {
			maxVersion = row.Version
		}
	}
	registryEmpty := len(rows) == 0

	versions := map[string]int{}
	nextLegacy := firstLegacyVersion
	for _, slot := range slots {
		if slot.svc == nil {
			continue
		}
		if _, done := versions[slot.keyID]; done {
			continue
		}
		version, known := byKeyID[slot.keyID]
		if !known {
			switch {
			case !registryEmpty:
				maxVersion++
				version = maxVersion
			case slot.role == roleCurrent:
				version = 1
			case slot.role == roleNext:
				version = 2
			default:
				version = nextLegacy
				nextLegacy++
			}
			if err := tx.Create(&models.EncryptionKeyVersion{
				Version: version, KeyID: slot.keyID, FirstSeenAt: now, LastSeenAt: now,
			}).Error; err != nil {
				return nil, fmt.Errorf("register key version %d: %w", version, err)
			}
			byKeyID[slot.keyID] = version
		} else if err := tx.Model(&models.EncryptionKeyVersion{}).
			Where(clause.Eq{Column: clause.Column{Name: "key_id"}, Value: slot.keyID}).
			Update("last_seen_at", now).Error; err != nil {
			return nil, fmt.Errorf("update key last seen: %w", err)
		}
		versions[slot.keyID] = version
	}
	return versions, nil
}

// current returns the version map in effect.
func (rs *RotationService) current() *keyState {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.state
}

// writeVersion is the key new data is written with: next when configured, else current.
func (s *keyState) writeVersion() int {
	if s.nextVersion != 0 {
		return s.nextVersion
	}
	return s.currentVersion
}

// decryptOrder lists the versions to try: the tag first, then current, next, and
// every other loaded version from the highest down.
func (s *keyState) decryptOrder(tag int) []int {
	order := make([]int, 0, len(s.byVersion))
	seen := map[int]bool{}
	add := func(v int) {
		if _, loaded := s.byVersion[v]; loaded && !seen[v] {
			seen[v] = true
			order = append(order, v)
		}
	}
	if tag > 0 {
		add(tag)
	}
	add(s.currentVersion)
	add(s.nextVersion)
	others := make([]int, 0, len(s.byVersion))
	for v := range s.byVersion {
		others = append(others, v)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(others)))
	for _, v := range others {
		add(v)
	}
	return order
}

// open decrypts ciphertext with the first key in decryptOrder that authenticates
// it, and returns that key's version.
func (s *keyState) open(ciphertext string, tag int) (plaintext []byte, version int, err error) {
	for _, v := range s.decryptOrder(tag) {
		plaintext, err := s.byVersion[v].Decrypt(ciphertext)
		if err == nil {
			return plaintext, v, nil
		}
	}
	return nil, 0, ErrNoLoadedKeyOpens
}

// DecryptWithVersion decrypts ciphertext tagged with keyVersion. The key for that
// version is tried first, then every other loaded key. AES-GCM authentication
// makes a wrong key fail rather than return garbage. A tag of 0 means untagged.
func (rs *RotationService) DecryptWithVersion(ciphertext string, keyVersion int) ([]byte, error) {
	if rs == nil {
		return nil, ErrNoEncryptionKey
	}
	plaintext, opened, err := rs.current().open(ciphertext, keyVersion)
	if err != nil {
		return nil, fmt.Errorf("decrypt with key version %d: %w", keyVersion, err)
	}
	if keyVersion > 0 && opened != keyVersion {
		logger.Log().WithFields(map[string]any{"key_version": opened, "tagged_version": keyVersion}).
			Debug("Stored secret opened with a key other than its tag; the next rotation corrects the tag")
	}
	return plaintext, nil
}

// EncryptWithCurrentKey encrypts plaintext with the next key when configured,
// otherwise the current key, and returns the version of the key used.
func (rs *RotationService) EncryptWithCurrentKey(plaintext []byte) (ciphertext string, keyVersion int, err error) {
	if rs == nil {
		return "", 0, ErrNoEncryptionKey
	}
	state := rs.current()
	version := state.writeVersion()
	ciphertext, err = state.byVersion[version].Encrypt(plaintext)
	if err != nil {
		return "", 0, fmt.Errorf("encrypt with key version %d: %w", version, err)
	}
	return ciphertext, version, nil
}

// Reconcile refreshes the key registry and the version map, then reports where
// the stored secrets are. It writes only to the key registry. Store rows are
// never changed here: wrong tags are harmless because reads fall back to every
// loaded key, and the next rotation corrects them.
func (rs *RotationService) Reconcile(ctx context.Context) (*RotationStatus, error) {
	if rs == nil {
		return nil, ErrNoEncryptionKey
	}
	if err := rs.reloadKeys(ctx); err != nil {
		return nil, fmt.Errorf("reconcile encryption keys: %w", err)
	}
	report, err := rs.scanUsage(ctx)
	if err != nil {
		return nil, err
	}
	status := rs.statusFrom(report)
	rs.logReport(report, status)
	return status, nil
}

// Rehydrate runs after a live restore replaced the key registry: it re-registers
// the loaded keys, swaps in the new version map and reconciles.
func (rs *RotationService) Rehydrate(ctx context.Context) error {
	if _, err := rs.Reconcile(ctx); err != nil {
		return fmt.Errorf("rehydrate encryption keys: %w", err)
	}
	return nil
}

// GetStatus verifies every stored secret by decrypting it and reports per key and
// per store what is still in use.
func (rs *RotationService) GetStatus(ctx context.Context) (*RotationStatus, error) {
	if rs == nil {
		return nil, ErrNoEncryptionKey
	}
	report, err := rs.scanUsage(ctx)
	if err != nil {
		return nil, err
	}
	return rs.statusFrom(report), nil
}

// RotateAll re-encrypts every stored secret to the rotation target inside one
// database write-lock transaction across all stores. The target is the next key
// when configured, otherwise the current key, which is only useful when some row
// still needs an older key. Any error rolls the whole run back.
func (rs *RotationService) RotateAll(ctx context.Context) (*RotationResult, error) {
	if rs == nil {
		return nil, ErrNoEncryptionKey
	}
	state := rs.current()
	target := state.writeVersion()
	started := time.Now()
	result := &RotationResult{TargetVersion: target, NewKeyVersion: target, StartedAt: started, Stores: []StoreRotation{}}

	err := database.WithWriteLock(rs.db.WithContext(ctx), &models.EncryptionKeyVersion{}, func(tx *gorm.DB) error {
		for _, store := range EncryptedStores {
			counts, err := rotateStore(tx, store, state, target)
			if err != nil {
				return fmt.Errorf("rotate %s: %w", store.Name, err)
			}
			result.addStore(counts)
		}
		if state.nextVersion == 0 && result.RotatedCount == 0 {
			return ErrRotationNotNeeded
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrRotationNotNeeded) {
			return nil, ErrRotationNotNeeded
		}
		return nil, fmt.Errorf("rotate stored secrets: %w", err)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(started).String()
	return result, nil
}

// addStore adds one store's counts to the run totals.
func (r *RotationResult) addStore(s StoreRotation) {
	r.Stores = append(r.Stores, s)
	r.TotalRecords += s.Total
	r.RotatedCount += s.Rotated
	r.AlreadyCurrentCount += s.AlreadyCurrent
	r.UnreadableCount += s.Unreadable
}

// rotateStore re-encrypts every row of one store that is not already on the target.
// A row that already opens with the target is only re-tagged when its tag is wrong.
// A row no loaded key opens is counted and left untouched.
func rotateStore(tx *gorm.DB, store EncryptedStore, state *keyState, target int) (StoreRotation, error) {
	counts := StoreRotation{Store: store.Name}
	if !tx.Migrator().HasTable(store.Table) {
		return counts, nil
	}
	targetSvc := state.byVersion[target]
	rows, err := store.loadRows(tx)
	if err != nil {
		return counts, fmt.Errorf("read rows: %w", err)
	}

	for _, row := range rows {
		counts.Total++
		if _, openErr := targetSvc.Decrypt(row.Cipher); openErr == nil {
			counts.AlreadyCurrent++
			if store.hasVersion() && row.Version != target {
				if err := store.updateVersion(tx, row.ID, target); err != nil {
					return counts, fmt.Errorf("fix key version of row %d: %w", row.ID, err)
				}
			}
			continue
		}

		plaintext, _, openErr := state.open(row.Cipher, row.Version)
		if openErr != nil {
			counts.Unreadable++
			logUnreadable(store.Name, row.ID)
			continue
		}
		newCipher, err := targetSvc.Encrypt(plaintext)
		if err != nil {
			return counts, fmt.Errorf("encrypt row %d: %w", row.ID, err)
		}
		// Self-check: the new ciphertext must open with the target key to the same bytes.
		check, err := targetSvc.Decrypt(newCipher)
		if err != nil || !bytes.Equal(check, plaintext) {
			return counts, fmt.Errorf("self-check failed for row %d", row.ID)
		}
		if err := store.updateRow(tx, row.ID, newCipher, target); err != nil {
			return counts, fmt.Errorf("write row %d: %w", row.ID, err)
		}
		counts.Rotated++
	}
	return counts, nil
}

// logUnreadable records a row no key can open. Only the store and row id are logged.
func logUnreadable(store string, id uint) {
	logger.Log().WithFields(map[string]any{"store": store, "row_id": id}).
		Warn("Stored secret could not be opened with any configured encryption key")
}

// scanUsage reads every store and records which loaded key opens each row.
func (rs *RotationService) scanUsage(ctx context.Context) (*usageReport, error) {
	state := rs.current()
	target := state.writeVersion()
	db := rs.db.WithContext(ctx)
	report := &usageReport{inUse: map[int]int{}}

	for _, store := range EncryptedStores {
		if !db.Migrator().HasTable(store.Table) {
			continue
		}
		rows, err := store.loadRows(db)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", store.Name, err)
		}
		usage := StoreUsage{Store: store.Name, Total: len(rows), ByVersion: map[int]int{}}
		for _, row := range rows {
			_, opened, openErr := state.open(row.Cipher, row.Version)
			if openErr != nil {
				usage.Unreadable++
				report.unreadable = append(report.unreadable, store.Name+"#"+strconv.FormatUint(uint64(row.ID), 10))
				continue
			}
			usage.ByVersion[opened]++
			report.inUse[opened]++
			if opened != target {
				report.misplaced++
			}
		}
		report.stores = append(report.stores, usage)
	}
	return report, nil
}

// statusFrom turns a usage scan into the status the API returns.
func (rs *RotationService) statusFrom(report *usageReport) *RotationStatus {
	state := rs.current()
	status := &RotationStatus{
		CurrentVersion:     state.currentVersion,
		NextKeyConfigured:  state.nextVersion != 0,
		NextVersion:        state.nextVersion,
		LegacyKeyVersions:  []int{},
		Keys:               []KeyUsage{},
		Stores:             report.stores,
		RotationNeeded:     report.misplaced > 0,
		ProvidersByVersion: map[int]int{},
	}
	if status.Stores == nil {
		status.Stores = []StoreUsage{}
	}
	status.UnreadableTotal = len(report.unreadable)

	legacySeen := map[int]bool{}
	for _, slot := range rs.slots {
		usage := KeyUsage{Slot: slot.name, Removable: true}
		if slot.svc != nil {
			version := state.versions[slot.keyID]
			usage.Version = version
			usage.KeyID = slot.keyID[:8]
			usage.Loaded = true
			usage.InUse = report.inUse[version]
			usage.Removable = version != state.currentVersion && usage.InUse == 0
			if slot.role == roleLegacy && version != state.currentVersion && version != state.nextVersion && !legacySeen[version] {
				legacySeen[version] = true
				status.LegacyKeyVersions = append(status.LegacyKeyVersions, version)
			}
		}
		status.Keys = append(status.Keys, usage)
	}
	sort.Ints(status.LegacyKeyVersions)
	status.LegacyKeyCount = len(status.LegacyKeyVersions)

	for _, usage := range report.stores {
		if usage.Store == "dns_providers" {
			readable := usage.Total - usage.Unreadable
			status.ProvidersByVersion = usage.ByVersion
			status.ProvidersOnCurrentVersion = usage.ByVersion[state.currentVersion]
			status.ProvidersOnOlderVersions = readable - status.ProvidersOnCurrentVersion
		}
	}
	return status
}

// logReport writes the reconcile summary: one Info line with the counts, one Warn
// when rows need a key other than the target, and one Warn per store with rows no
// key can open.
func (rs *RotationService) logReport(report *usageReport, status *RotationStatus) {
	parts := make([]string, 0, len(status.Stores))
	for _, s := range status.Stores {
		parts = append(parts, s.Store+"="+strconv.Itoa(s.Total))
	}
	logger.Log().WithFields(map[string]any{
		"current_version": status.CurrentVersion,
		"stores":          strings.Join(parts, " "),
	}).Info("Encryption key reconcile complete")

	if report.misplaced > 0 {
		logger.Log().WithFields(map[string]any{
			"rows":           report.misplaced,
			"target_version": rs.current().writeVersion(),
		}).Warn("Stored secrets use an encryption key other than the rotation target; rotation corrects them")
	}
	unreadableByStore := map[string]int{}
	for _, id := range report.unreadable {
		unreadableByStore[strings.SplitN(id, "#", 2)[0]]++
	}
	for _, s := range status.Stores {
		if n := unreadableByStore[s.Store]; n > 0 {
			logger.Log().WithFields(map[string]any{"store": s.Store, "count": n}).
				Warn("Stored secrets could not be opened with any configured encryption key")
		}
	}
}

// ValidateKeyConfiguration round-trips a test value through every configured key.
// A slot whose value failed to load is reported by name as well.
func (rs *RotationService) ValidateKeyConfiguration() error {
	if rs == nil {
		return ErrNoEncryptionKey
	}
	testData := []byte("validation_test")
	var failed []string
	for _, slot := range rs.slots {
		if slot.svc == nil || !roundTrips(slot.svc, testData) {
			failed = append(failed, slot.name)
		}
	}
	if len(failed) > 0 {
		return &KeyValidationError{Slots: failed}
	}
	return nil
}

// roundTrips reports whether svc encrypts and decrypts data back to itself.
func roundTrips(svc *EncryptionService, data []byte) bool {
	ciphertext, err := svc.Encrypt(data)
	if err != nil {
		return false
	}
	plaintext, err := svc.Decrypt(ciphertext)
	return err == nil && bytes.Equal(plaintext, data)
}

// GenerateNewKey generates a new random 32-byte encryption key and returns it as base64.
// This is a utility function for administrators to generate keys for rotation.
func GenerateNewKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
