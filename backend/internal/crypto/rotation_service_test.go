package crypto

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// rotationFixture is a file-backed SQLite database with the single-connection
// pool the server uses, so a query issued outside the rotation transaction
// would block the test instead of passing by accident.
type rotationFixture struct {
	t  *testing.T
	db *gorm.DB
}

func newRotationFixture(t *testing.T) *rotationFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rotation.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.AutoMigrate(
		&models.EncryptionKeyVersion{},
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
		&models.RemoteStorageTarget{},
		&models.SSLCertificate{},
		&models.TunnelConfig{},
		&models.Setting{},
	))
	return &rotationFixture{t: t, db: db}
}

// newService builds a rotation service over the fixture database.
func (f *rotationFixture) newService(keys config.EncryptionKeys) *RotationService {
	f.t.Helper()
	rs, err := NewRotationService(f.db, keys)
	require.NoError(f.t, err)
	return rs
}

// key returns a fresh random base64 key.
func key(t *testing.T) string {
	t.Helper()
	k, err := GenerateNewKey()
	require.NoError(t, err)
	return k
}

// service returns an EncryptionService for a base64 key.
func service(t *testing.T, keyB64 string) *EncryptionService {
	t.Helper()
	svc, err := NewEncryptionService(keyB64)
	require.NoError(t, err)
	return svc
}

// seal encrypts plaintext with svc.
func seal(t *testing.T, svc *EncryptionService, plaintext string) string {
	t.Helper()
	ct, err := svc.Encrypt([]byte(plaintext))
	require.NoError(t, err)
	return ct
}

// seedAll writes one row into every store, encrypted with svc and tagged with tag.
// It returns the row id per store name.
func (f *rotationFixture) seedAll(svc *EncryptionService, tag int) map[string]uint {
	f.t.Helper()
	ids := map[string]uint{}

	provider := models.DNSProvider{UUID: uuid.NewString(), Name: "p", ProviderType: "cloudflare"}
	require.NoError(f.t, f.db.Create(&provider).Error)
	ids["dns_providers"] = provider.ID
	f.setCipher("dns_providers", provider.ID, seal(f.t, svc, "provider"), tag)

	credential := models.DNSProviderCredential{UUID: uuid.NewString(), DNSProviderID: provider.ID, Label: "c"}
	require.NoError(f.t, f.db.Create(&credential).Error)
	ids["dns_provider_credentials"] = credential.ID
	f.setCipher("dns_provider_credentials", credential.ID, seal(f.t, svc, "credential"), tag)

	target := models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "r", Type: "s3"}
	require.NoError(f.t, f.db.Create(&target).Error)
	ids["remote_storage_targets"] = target.ID
	f.setCipher("remote_storage_targets", target.ID, seal(f.t, svc, "remote"), tag)

	cert := models.SSLCertificate{UUID: uuid.NewString(), Name: "c"}
	require.NoError(f.t, f.db.Create(&cert).Error)
	ids["certificate_keys"] = cert.ID
	f.setCipher("certificate_keys", cert.ID, seal(f.t, svc, "private-key"), tag)

	tunnel := models.TunnelConfig{Name: "t", Provider: models.ProviderCloudflare}
	require.NoError(f.t, f.db.Create(&tunnel).Error)
	ids["tunnels"] = tunnel.ID
	f.setCipher("tunnels", tunnel.ID, seal(f.t, svc, "tunnel"), tag)

	setting := models.Setting{Key: EncryptedSettingKeys[0], Type: "string", Category: "backup"}
	require.NoError(f.t, f.db.Where(models.Setting{Key: setting.Key}).Attrs(setting).FirstOrCreate(&setting).Error)
	f.setCipher("setting:"+EncryptedSettingKeys[0], setting.ID, seal(f.t, svc, "passphrase"), 0)
	ids["setting:"+EncryptedSettingKeys[0]] = setting.ID
	return ids
}

// setCipher overwrites one row's cipher and tag directly, for fixtures that need a
// specific tag or a ciphertext from a key that is not configured.
func (f *rotationFixture) setCipher(storeName string, id uint, cipher string, tag int) {
	f.t.Helper()
	store := storeByName(f.t, storeName)
	require.NoError(f.t, f.db.Transaction(func(tx *gorm.DB) error {
		return store.updateRow(tx, id, cipher, tag)
	}))
}

// storeByName returns the registry entry with the given name.
func storeByName(t *testing.T, name string) EncryptedStore {
	t.Helper()
	for _, s := range EncryptedStores {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no store named %q", name)
	return EncryptedStore{}
}

// snapshot returns every store row's cipher and tag, keyed by store and id.
func (f *rotationFixture) snapshot() map[string]string {
	f.t.Helper()
	out := map[string]string{}
	for _, s := range EncryptedStores {
		rows, err := s.loadRows(f.db)
		require.NoError(f.t, err)
		for _, r := range rows {
			out[s.Name+"/"+strconv.FormatUint(uint64(r.ID), 10)] = r.Cipher + "|" + strconv.Itoa(r.Version)
		}
	}
	return out
}

// registryVersions returns key_id -> version for every registry row.
func (f *rotationFixture) registryVersions() map[string]int {
	f.t.Helper()
	var rows []models.EncryptionKeyVersion
	require.NoError(f.t, f.db.Find(&rows).Error)
	out := map[string]int{}
	for _, r := range rows {
		out[r.KeyID] = r.Version
	}
	return out
}

func TestNewRotationService_RequiresCurrentKey(t *testing.T) {
	f := newRotationFixture(t)
	_, err := NewRotationService(f.db, config.EncryptionKeys{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CHARON_ENCRYPTION_KEY is required")
}

func TestNewRotationService_RejectsInvalidCurrentKey(t *testing.T) {
	f := newRotationFixture(t)
	_, err := NewRotationService(f.db, config.EncryptionKeys{Current: "not-base64!"})
	require.Error(t, err)
}

func TestKeyID_SameForDifferentBase64Text(t *testing.T) {
	// Non-canonical trailing bits decode to the same 32 bytes, so the id must match.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	canonical := key(t)
	raw, err := base64.StdEncoding.DecodeString(canonical)
	require.NoError(t, err)
	alt := []byte(canonical)
	idx := strings.IndexByte(alphabet, alt[42])
	require.GreaterOrEqual(t, idx, 0)
	alt[42] = alphabet[idx^3] // flips the two padding bits of the last data character
	altRaw, err := base64.StdEncoding.DecodeString(string(alt))
	require.NoError(t, err)
	require.Equal(t, raw, altRaw)
	require.NotEqual(t, canonical, string(alt))

	assert.Equal(t, keyIDFor(service(t, canonical)), keyIDFor(service(t, string(alt))))
	assert.Len(t, keyIDFor(service(t, canonical)), 32)
}

func TestNewRotationService_EmptyRegistryFillsCurrentNextThenLegacy(t *testing.T) {
	f := newRotationFixture(t)
	current, next, v1, v3 := key(t), key(t), key(t), key(t)
	f.newService(config.EncryptionKeys{Current: current, Next: next, Legacy: map[int]string{1: v1, 3: v3}})

	versions := f.registryVersions()
	assert.Equal(t, 1, versions[keyIDFor(service(t, current))])
	assert.Equal(t, 2, versions[keyIDFor(service(t, next))])
	assert.Equal(t, 3, versions[keyIDFor(service(t, v1))], "first loaded legacy slot takes the first free number after next")
	assert.Equal(t, 4, versions[keyIDFor(service(t, v3))])
}

func TestNewRotationService_ExistingRegistryIsNotRenumbered(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	f.newService(config.EncryptionKeys{Current: current, Next: next})
	before := f.registryVersions()

	// A later start with the same keys in other slots must keep the same numbers.
	rs := f.newService(config.EncryptionKeys{Current: next, Legacy: map[int]string{1: current}})
	assert.Equal(t, before, f.registryVersions())
	status, err := rs.GetStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, status.CurrentVersion)
}

func TestPromotion_KeyFollowsItsVersionIntoSlotV1(t *testing.T) {
	f := newRotationFixture(t)
	oldKey, newKey := key(t), key(t)

	rs := f.newService(config.EncryptionKeys{Current: oldKey, Next: newKey})
	oldVersion := rs.versionOfKey(t, oldKey)
	ct := seal(t, service(t, oldKey), "tagged with the old key")

	// Promotion: new key becomes current, old key moves to the V1 slot.
	promoted := f.newService(config.EncryptionKeys{Current: newKey, Legacy: map[int]string{1: oldKey}})
	assert.Equal(t, oldVersion, promoted.versionOfKey(t, oldKey))

	plain, err := promoted.DecryptWithVersion(ct, oldVersion)
	require.NoError(t, err)
	assert.Equal(t, "tagged with the old key", string(plain))
}

// versionOfKey returns the registry version the service assigned to keyB64.
func (rs *RotationService) versionOfKey(t *testing.T, keyB64 string) int {
	t.Helper()
	state := rs.current()
	id := keyIDFor(service(t, keyB64))
	for version, svc := range state.byVersion {
		if keyIDFor(svc) == id {
			return version
		}
	}
	t.Fatalf("key has no version in this service")
	return 0
}

func TestDecryptWithVersion_FallsBackToEveryLoadedKey(t *testing.T) {
	buf := captureLogs(t)
	f := newRotationFixture(t)
	current, next, legacy := key(t), key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next, Legacy: map[int]string{2: legacy}})

	for _, k := range []string{current, next, legacy} {
		ct := seal(t, service(t, k), "fallback")
		// Tag 99 is wrong for every key: the service must still find the right one.
		plain, err := rs.DecryptWithVersion(ct, 99)
		require.NoError(t, err)
		assert.Equal(t, "fallback", string(plain))
	}
	assert.Contains(t, buf.String(), "key_version")
	assert.NotContains(t, buf.String(), current)
	assert.NotContains(t, buf.String(), legacy)
}

func TestDecryptWithVersion_UnknownKeyFails(t *testing.T) {
	f := newRotationFixture(t)
	rs := f.newService(config.EncryptionKeys{Current: key(t)})
	ct := seal(t, service(t, key(t)), "stranger")
	_, err := rs.DecryptWithVersion(ct, 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoLoadedKeyOpens)
}

func TestDecryptWithVersion_UntaggedZeroTriesAllKeys(t *testing.T) {
	f := newRotationFixture(t)
	legacy := key(t)
	rs := f.newService(config.EncryptionKeys{Current: key(t), Legacy: map[int]string{1: legacy}})
	plain, err := rs.DecryptWithVersion(seal(t, service(t, legacy), "untagged"), 0)
	require.NoError(t, err)
	assert.Equal(t, "untagged", string(plain))
}

func TestDuplicateAndInvalidKeys(t *testing.T) {
	buf := captureLogs(t)
	f := newRotationFixture(t)
	current := key(t)
	rs := f.newService(config.EncryptionKeys{
		Current: current,
		Next:    current, // next equal to current: not configured
		Legacy:  map[int]string{1: current, 2: "!!not base64!!", 3: key(t)},
	})

	status, err := rs.GetStatus(context.Background())
	require.NoError(t, err)
	assert.False(t, status.NextKeyConfigured)
	assert.Equal(t, 1, status.LegacyKeyCount, "legacy slot equal to current adds no version; the invalid slot is not loaded")

	var invalid, loaded int
	for _, k := range status.Keys {
		if k.Loaded {
			loaded++
		} else {
			invalid++
			assert.Equal(t, "CHARON_ENCRYPTION_KEY_V2", k.Slot)
		}
	}
	assert.Equal(t, 1, invalid)
	assert.Equal(t, 3, loaded, "current, duplicate V1 and V3")
	assert.Contains(t, buf.String(), "CHARON_ENCRYPTION_KEY_V2")
	assert.NotContains(t, buf.String(), "!!not base64!!")
	assert.NotContains(t, buf.String(), current)
}

func TestEncryptWithCurrentKey_UsesNextKeyWhenConfigured(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})

	ct, version, err := rs.EncryptWithCurrentKey([]byte("new write"))
	require.NoError(t, err)
	assert.Equal(t, rs.versionOfKey(t, next), version)
	plain, err := service(t, next).Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, "new write", string(plain))

	rsCurrent := f.newService(config.EncryptionKeys{Current: current})
	_, version, err = rsCurrent.EncryptWithCurrentKey([]byte("x"))
	require.NoError(t, err)
	assert.Equal(t, rsCurrent.versionOfKey(t, current), version)
}

func TestNilRotationService_BehavesAsNoKey(t *testing.T) {
	var rs *RotationService // typed nil: must not panic
	_, _, err := rs.EncryptWithCurrentKey([]byte("x"))
	assert.ErrorIs(t, err, ErrNoEncryptionKey)
	_, err = rs.DecryptWithVersion("abc", 1)
	assert.ErrorIs(t, err, ErrNoEncryptionKey)
	_, err = rs.GetStatus(context.Background())
	assert.ErrorIs(t, err, ErrNoEncryptionKey)
}

func TestReconcile_WritesOnlyRegistryAndLeavesRowsByteForByte(t *testing.T) {
	buf := captureLogs(t)
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	f.seedAll(service(t, current), rs.versionOfKey(t, current))
	// Rows that no configured key opens (E7): reconcile reports them and leaves them alone.
	f.seedAll(service(t, key(t)), 1)
	before := f.snapshot()

	_, err := rs.Reconcile(context.Background())
	require.NoError(t, err)

	assert.Equal(t, before, f.snapshot(), "reconcile must not change any store row")
	assert.Contains(t, buf.String(), "could not be opened")
	assert.NotContains(t, buf.String(), current)
}

func TestReconcile_UpgradeFromOldTagsFillsRegistryOnly(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	// Rows written before this release: tag 1 for current, tag 2 for next.
	f.seedAll(service(t, current), 1)
	cipherOnNext := f.seedAll(service(t, next), 2)
	_ = cipherOnNext
	require.Equal(t, int64(0), countRows(t, f.db, &models.EncryptionKeyVersion{}))

	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	before := f.snapshot()
	_, err := rs.Reconcile(context.Background())
	require.NoError(t, err)
	assert.Equal(t, before, f.snapshot())
	assert.Equal(t, int64(2), countRows(t, f.db, &models.EncryptionKeyVersion{}))
}

func countRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(model).Count(&n).Error)
	return n
}

func TestRotateAll_RotatesEveryStoreAndSettingToNextKey(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	ids := f.seedAll(service(t, current), 1)

	result, err := rs.RotateAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, rs.versionOfKey(t, next), result.TargetVersion)
	assert.Equal(t, len(EncryptedStores), len(result.Stores))
	assert.Equal(t, len(ids), result.TotalRecords)
	assert.Equal(t, len(ids), result.RotatedCount)
	assert.Equal(t, 0, result.AlreadyCurrentCount)
	assert.Equal(t, 0, result.UnreadableCount)
	for _, s := range result.Stores {
		assert.Equal(t, 1, s.Total, s.Store)
		assert.Equal(t, 1, s.Rotated, s.Store)
	}

	// Every row now opens with the next key and carries its version tag.
	nextSvc := service(t, next)
	for _, s := range EncryptedStores {
		rows, err := s.loadRows(f.db)
		require.NoError(t, err)
		for _, r := range rows {
			_, err := nextSvc.Decrypt(r.Cipher)
			assert.NoError(t, err, s.Name)
			if s.hasVersion() {
				assert.Equal(t, result.TargetVersion, r.Version, s.Name)
			}
		}
	}
}

func TestRotateAll_RerunIsIdempotent(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	f.seedAll(service(t, current), 1)

	_, err := rs.RotateAll(context.Background())
	require.NoError(t, err)
	before := f.snapshot()

	second, err := rs.RotateAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, second.RotatedCount)
	assert.Equal(t, second.TotalRecords, second.AlreadyCurrentCount)
	assert.Equal(t, before, f.snapshot(), "a second run changes nothing")
}

func TestRotateAll_FixesWrongTagWithoutReencrypting(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	ids := f.seedAll(service(t, next), 1) // already on next, wrong tag

	result, err := rs.RotateAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, result.RotatedCount)
	assert.Equal(t, len(ids), result.AlreadyCurrentCount)

	rows, err := storeByName(t, "dns_providers").loadRows(f.db)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, rs.versionOfKey(t, next), rows[0].Version)
}

func TestRotateAll_RollsBackEveryStoreWhenOneRowFails(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	f.seedAll(service(t, current), 1)
	before := f.snapshot()

	// Fail on the fourth row write, after several stores were already rewritten.
	writes := 0
	const hook = "test:fail-fourth-write"
	require.NoError(t, f.db.Callback().Update().Before("gorm:update").Register(hook, func(tx *gorm.DB) {
		writes++
		if writes == 4 {
			_ = tx.AddError(errors.New("injected write failure"))
		}
	}))
	t.Cleanup(func() { _ = f.db.Callback().Update().Remove(hook) })

	_, err := rs.RotateAll(context.Background())
	require.Error(t, err)
	assert.Equal(t, before, f.snapshot(), "a failed rotation leaves every row as it was")
}

func TestRotateAll_NothingToDoWithoutNextKeyReturnsNotNeeded(t *testing.T) {
	f := newRotationFixture(t)
	current := key(t)
	rs := f.newService(config.EncryptionKeys{Current: current})
	f.seedAll(service(t, current), 1)
	before := f.snapshot()

	_, err := rs.RotateAll(context.Background())
	assert.ErrorIs(t, err, ErrRotationNotNeeded)
	assert.Equal(t, before, f.snapshot())
}

func TestRotateAll_WithoutNextReencryptsToCurrentWhenOlderRowsExist(t *testing.T) {
	f := newRotationFixture(t)
	oldKey, newKey := key(t), key(t)
	f.newService(config.EncryptionKeys{Current: oldKey, Next: newKey})
	// Promoted without rotating: rows still on the old key, which now sits in V1.
	rs := f.newService(config.EncryptionKeys{Current: newKey, Legacy: map[int]string{1: oldKey}})
	f.seedAll(service(t, oldKey), rs.versionOfKey(t, oldKey))

	result, err := rs.RotateAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, rs.versionOfKey(t, newKey), result.TargetVersion)
	assert.Equal(t, len(EncryptedStores), result.RotatedCount)
}

func TestRotateAll_SkipsUnreadableRowsAndLeavesThemUntouched(t *testing.T) {
	captureLogs(t)
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	f.seedAll(service(t, key(t)), 1) // no loaded key opens these
	before := f.snapshot()

	result, err := rs.RotateAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, result.RotatedCount)
	assert.Equal(t, len(EncryptedStores), result.UnreadableCount)
	assert.Equal(t, before, f.snapshot())
}

func TestRotateAll_ConcurrentCallsRotateEachRowOnce(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	ids := f.seedAll(service(t, current), 1)

	var wg sync.WaitGroup
	results := make([]*RotationResult, 2)
	errs := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = rs.RotateAll(context.Background())
		}(i)
	}
	wg.Wait()

	for i := range results {
		require.NoError(t, errs[i])
	}
	assert.Equal(t, len(ids), results[0].RotatedCount+results[1].RotatedCount)
}

func TestGetStatus_VerifiesByDecryptionAndMarksRemovableKeys(t *testing.T) {
	f := newRotationFixture(t)
	current, next, legacy := key(t), key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next, Legacy: map[int]string{1: legacy}})

	// Rows on current, but tagged as next: the tag is wrong, the key is right.
	ids := f.seedAll(service(t, current), rs.versionOfKey(t, next))
	status, err := rs.GetStatus(context.Background())
	require.NoError(t, err)

	assert.Equal(t, rs.versionOfKey(t, current), status.CurrentVersion)
	assert.Equal(t, rs.versionOfKey(t, next), status.NextVersion)
	assert.True(t, status.NextKeyConfigured)
	assert.True(t, status.RotationNeeded, "rows open with current, the target is next")
	assert.Equal(t, 0, status.UnreadableTotal)
	assert.Equal(t, []int{rs.versionOfKey(t, legacy)}, status.LegacyKeyVersions)

	providers := status.Stores[0]
	assert.Equal(t, "dns_providers", providers.Store)
	assert.Equal(t, 1, providers.Total)
	assert.Equal(t, map[int]int{rs.versionOfKey(t, current): 1}, providers.ByVersion)
	assert.Equal(t, 0, status.ProvidersOnOlderVersions, "the row opens with the current key, so it is on the current version")

	byName := map[string]KeyUsage{}
	for _, k := range status.Keys {
		byName[k.Slot] = k
	}
	assert.False(t, byName["CHARON_ENCRYPTION_KEY"].Removable)
	assert.Equal(t, len(ids), byName["CHARON_ENCRYPTION_KEY"].InUse)
	assert.True(t, byName["CHARON_ENCRYPTION_KEY_V1"].Removable)
	assert.Equal(t, 0, byName["CHARON_ENCRYPTION_KEY_V1"].InUse)
	assert.Len(t, byName["CHARON_ENCRYPTION_KEY"].KeyID, 8)
}

func TestGetStatus_ReportsUnreadableRows(t *testing.T) {
	captureLogs(t)
	f := newRotationFixture(t)
	rs := f.newService(config.EncryptionKeys{Current: key(t)})
	f.seedAll(service(t, key(t)), 1)

	status, err := rs.GetStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(EncryptedStores), status.UnreadableTotal)
	assert.False(t, status.RotationNeeded)
}

func TestStoreMissingTableIsSkipped(t *testing.T) {
	f := newRotationFixture(t)
	require.NoError(t, f.db.Migrator().DropTable("remote_storage_targets"))
	rs := f.newService(config.EncryptionKeys{Current: key(t)})
	status, err := rs.GetStatus(context.Background())
	require.NoError(t, err)
	for _, s := range status.Stores {
		assert.NotEqual(t, "remote_storage_targets", s.Store)
	}
}

func TestRehydrate_ReregistersAndReloadsWhileDecryptsRun(t *testing.T) {
	f := newRotationFixture(t)
	current, next := key(t), key(t)
	rs := f.newService(config.EncryptionKeys{Current: current, Next: next})
	nextVersion := rs.versionOfKey(t, next)
	ct := seal(t, service(t, next), "restored")

	// A live restore replaced the registry with an empty table (old backup).
	require.NoError(t, f.db.Where("1 = 1").Delete(&models.EncryptionKeyVersion{}).Error)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = rs.DecryptWithVersion(ct, nextVersion)
				}
			}
		}()
	}
	require.NoError(t, rs.Rehydrate(context.Background()))
	close(stop)
	wg.Wait()

	assert.Equal(t, nextVersion, rs.versionOfKey(t, next))
	_, version, err := rs.EncryptWithCurrentKey([]byte("after"))
	require.NoError(t, err)
	assert.Equal(t, nextVersion, version)
}

func TestValidateKeyConfiguration_RoundTripsEveryLoadedKey(t *testing.T) {
	f := newRotationFixture(t)
	rs := f.newService(config.EncryptionKeys{Current: key(t), Next: key(t), Legacy: map[int]string{1: key(t)}})
	require.NoError(t, rs.ValidateKeyConfiguration())

	var nilRS *RotationService
	assert.ErrorIs(t, nilRS.ValidateKeyConfiguration(), ErrNoEncryptionKey)
}

func TestGenerateNewKey_IsDecodableThirtyTwoBytes(t *testing.T) {
	k, err := GenerateNewKey()
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(k)
	require.NoError(t, err)
	assert.Len(t, raw, 32)
}

// captureLogs routes the shared logger to a buffer for the rest of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger.Init(true, &buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })
	return &buf
}
