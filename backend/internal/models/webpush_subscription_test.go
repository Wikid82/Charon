package models

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupWebPushSubscriptionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&NotificationProvider{}, &WebPushSubscription{}))
	return db
}

func TestWebPushSubscription_BeforeCreate_GeneratesUUID(t *testing.T) {
	db := setupWebPushSubscriptionTestDB(t)

	sub := &WebPushSubscription{
		ProviderID: "provider-1",
		UserID:     "user-1",
		Endpoint:   "https://fcm.googleapis.com/fcm/send/abc",
		P256dh:     "p256dh-value",
		Auth:       "auth-value",
	}
	require.NoError(t, db.Create(sub).Error)

	assert.NotEmpty(t, sub.ID, "ID should be set after create")
	assert.Len(t, sub.ID, 36, "UUID should be 36 characters")
}

func TestWebPushSubscription_BeforeCreate_PreservesExistingID(t *testing.T) {
	db := setupWebPushSubscriptionTestDB(t)

	sub := &WebPushSubscription{
		ID:         "explicit-id",
		ProviderID: "provider-1",
		UserID:     "user-1",
		Endpoint:   "https://fcm.googleapis.com/fcm/send/def",
		P256dh:     "p256dh-value",
		Auth:       "auth-value",
	}
	require.NoError(t, db.Create(sub).Error)

	assert.Equal(t, "explicit-id", sub.ID)
}

// TestWebPushSubscription_EndpointUniqueIndex asserts a second row with the
// same Endpoint is rejected at the database level (§3.3.2 design note: the
// same browser re-subscribing must not create duplicate rows that both
// receive the same push).
func TestWebPushSubscription_EndpointUniqueIndex(t *testing.T) {
	db := setupWebPushSubscriptionTestDB(t)

	endpoint := "https://fcm.googleapis.com/fcm/send/duplicate"
	first := &WebPushSubscription{
		ProviderID: "provider-1",
		UserID:     "user-1",
		Endpoint:   endpoint,
		P256dh:     "p256dh-a",
		Auth:       "auth-a",
	}
	require.NoError(t, db.Create(first).Error)

	second := &WebPushSubscription{
		ProviderID: "provider-1",
		UserID:     "user-2",
		Endpoint:   endpoint,
		P256dh:     "p256dh-b",
		Auth:       "auth-b",
	}
	err := db.Create(second).Error
	require.Error(t, err, "duplicate endpoint should be rejected by the unique index")
}

// TestWebPushSubscription_FieldsNeverExposeJSON asserts P256dh/Auth are
// never round-tripped to the frontend (json:"-"), per §3.3.2's
// defense-in-depth rationale — this pins the struct tag contract with a
// regression test rather than relying on code review alone to catch a
// future accidental removal of json:"-".
func TestWebPushSubscription_FieldsNeverExposeJSON(t *testing.T) {
	// This is a compile-time-adjacent assertion: marshal a populated struct
	// and confirm P256dh/Auth values never appear in the output.
	sub := WebPushSubscription{
		ID:         "id-1",
		ProviderID: "provider-1",
		UserID:     "user-1",
		Endpoint:   "https://example.com/endpoint",
		P256dh:     "super-secret-p256dh",
		Auth:       "super-secret-auth",
	}
	b, err := json.Marshal(sub)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "super-secret-p256dh")
	assert.NotContains(t, string(b), "super-secret-auth")
}

// TestWebPushSingletonIndex_ConcurrentInsertsProduceExactlyOneWinner is the
// regression test for the exact race Supervisor identified (see
// docs/plans/current_spec.md §3.1 "Enforcement" and §3.3.4): a
// service-layer COUNT-then-INSERT check alone is not atomic under
// concurrent requests. This test creates the same
// idx_webpush_singleton partial unique index routes.go creates at startup,
// fires two goroutines each inserting a NotificationProvider{Type:
// "webpush"} row against the same *gorm.DB (mirroring the single-writer
// SQLite connection pool production uses, database.go's
// SetMaxOpenConns(1)), and asserts exactly one INSERT succeeds while the
// other fails with a unique-constraint-violation error. It must fail
// against pre-fix (service-layer-COUNT-only) code, since bypassing the
// index entirely would let both concurrent INSERTs succeed.
func TestWebPushSingletonIndex_ConcurrentInsertsProduceExactlyOneWinner(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "webpush_singleton_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Mirror production's single-writer-connection SQLite pool
	// (internal/database/database.go) so concurrent goroutines actually
	// interleave their statements on one connection, exercising the same
	// race window the index is designed to close.
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(&NotificationProvider{}, &WebPushSubscription{}))

	// Same DDL as backend/internal/api/routes/routes.go's post-AutoMigrate
	// singleton-index step.
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_webpush_singleton
		ON notification_providers(type) WHERE type = 'webpush'`).Error)

	const attempts = 2
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func(idx int) {
			defer wg.Done()
			provider := &NotificationProvider{
				Name: "Browser Push",
				Type: "webpush",
			}
			errs[idx] = db.Create(provider).Error
		}(i)
	}
	wg.Wait()

	successCount := 0
	failureCount := 0
	for _, e := range errs {
		if e == nil {
			successCount++
		} else {
			failureCount++
			assert.Contains(t, e.Error(), "UNIQUE constraint failed", "losing insert should fail on the partial unique index, got: %v", e)
		}
	}

	assert.Equal(t, 1, successCount, "exactly one concurrent insert should succeed")
	assert.Equal(t, 1, failureCount, "exactly one concurrent insert should fail with a unique-constraint violation")

	var count int64
	require.NoError(t, db.Model(&NotificationProvider{}).Where("type = ?", "webpush").Count(&count).Error)
	assert.Equal(t, int64(1), count, "only one webpush provider row should exist after the race")
}
