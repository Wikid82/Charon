package services

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupStatsServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "stats_service_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.RequestLog{},
		&models.SSLCertificate{},
		&models.ProxyHost{},
	))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedRequestLogs(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()

	logs := []models.RequestLog{
		// within last 24h
		{HostID: "host-a", Timestamp: now.Add(-1 * time.Hour), Method: "GET", StatusCode: 200, BytesSent: 100, DurationMs: 10},
		{HostID: "host-a", Timestamp: now.Add(-2 * time.Hour), Method: "POST", StatusCode: 201, BytesSent: 200, DurationMs: 20},
		{HostID: "host-b", Timestamp: now.Add(-3 * time.Hour), Method: "GET", StatusCode: 404, BytesSent: 50, DurationMs: 5},
		// within last 7d but not 24h
		{HostID: "host-a", Timestamp: now.Add(-48 * time.Hour), Method: "GET", StatusCode: 500, BytesSent: 300, DurationMs: 30},
		{HostID: "host-b", Timestamp: now.Add(-72 * time.Hour), Method: "DELETE", StatusCode: 204, BytesSent: 0, DurationMs: 8},
		// within last 30d but not 7d
		{HostID: "host-c", Timestamp: now.Add(-10 * 24 * time.Hour), Method: "GET", StatusCode: 200, BytesSent: 400, DurationMs: 15},
		{HostID: "host-c", Timestamp: now.Add(-20 * 24 * time.Hour), Method: "GET", StatusCode: 200, BytesSent: 500, DurationMs: 25},
	}
	require.NoError(t, db.Create(&logs).Error)
}

// TestStatsService_GetSummary verifies correct request counts per window.
func TestStatsService_GetSummary(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	summary, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(3), summary.RequestsLast24h)
	assert.Equal(t, int64(5), summary.RequestsLast7d)
	assert.Equal(t, int64(7), summary.RequestsLast30d)
}

// TestStatsService_GetSummary_TTLCache verifies the cached result is returned on second call.
func TestStatsService_GetSummary_TTLCache(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	first, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	// Insert a new row — if cache is working, second call should return same result.
	extra := models.RequestLog{
		HostID:     "host-d",
		Timestamp:  time.Now().UTC().Add(-30 * time.Minute),
		Method:     "GET",
		StatusCode: 200,
		BytesSent:  10,
		DurationMs: 1,
	}
	require.NoError(t, db.Create(&extra).Error)

	second, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	// Cache should serve the same values.
	assert.Equal(t, first, second)
}

// TestStatsService_GetTopHosts_ValidPeriod verifies top hosts ordering.
func TestStatsService_GetTopHosts_ValidPeriod(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	hosts, err := svc.GetTopHosts(context.Background(), "30d", 10)
	require.NoError(t, err)

	require.NotEmpty(t, hosts)
	// host-a has 3 requests in 30d, host-b has 2, host-c has 2.
	assert.Equal(t, "host-a", hosts[0].HostID)
	assert.Equal(t, int64(3), hosts[0].Count)
}

// TestStatsService_GetTopHosts_InvalidPeriod verifies rejection of unknown period.
func TestStatsService_GetTopHosts_InvalidPeriod(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	svc := NewStatsService(db)

	_, err := svc.GetTopHosts(context.Background(), "99d", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid period")
}

// TestStatsService_GetTopHosts_LimitCap verifies limit is capped at 50.
func TestStatsService_GetTopHosts_LimitCap(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	// Even with limit=100, only 3 unique hosts exist — should not error.
	hosts, err := svc.GetTopHosts(context.Background(), "30d", 100)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(hosts), 50)
}

// TestStatsService_GetTopHosts_PopulatesHostnameFromProxyHost verifies that the
// hostname is joined in from the matching proxy_hosts row rather than left blank.
func TestStatsService_GetTopHosts_PopulatesHostnameFromProxyHost(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:        "host-a",
		Name:        "API Server",
		DomainNames: "api.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}).Error)
	svc := NewStatsService(db)

	hosts, err := svc.GetTopHosts(context.Background(), "30d", 10)
	require.NoError(t, err)

	require.NotEmpty(t, hosts)
	assert.Equal(t, "host-a", hosts[0].HostID)
	assert.Equal(t, "API Server", hosts[0].Hostname)
}

// TestStatsService_GetTopHosts_FallsBackToHostIDWhenHostMissing verifies that a
// host with no matching proxy_hosts row (e.g. deleted host) falls back to host_id
// instead of returning a blank hostname.
func TestStatsService_GetTopHosts_FallsBackToHostIDWhenHostMissing(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	hosts, err := svc.GetTopHosts(context.Background(), "30d", 10)
	require.NoError(t, err)

	require.NotEmpty(t, hosts)
	for _, h := range hosts {
		assert.NotEmpty(t, h.Hostname)
		assert.Equal(t, h.HostID, h.Hostname)
	}
}

// TestStatsService_GetStatusDistribution_ValidPeriod verifies status code grouping.
func TestStatsService_GetStatusDistribution_ValidPeriod(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	dist, err := svc.GetStatusDistribution(context.Background(), "30d")
	require.NoError(t, err)

	require.NotEmpty(t, dist)

	// Collect all codes returned.
	codes := make(map[int]int64)
	for _, s := range dist {
		codes[s.Code] = s.Count
	}
	assert.Equal(t, int64(3), codes[200])
	assert.Equal(t, int64(1), codes[201])
	assert.Equal(t, int64(1), codes[404])
	assert.Equal(t, int64(1), codes[500])
	assert.Equal(t, int64(1), codes[204])
}

// TestStatsService_GetStatusDistribution_InvalidPeriod verifies rejection.
func TestStatsService_GetStatusDistribution_InvalidPeriod(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	svc := NewStatsService(db)

	_, err := svc.GetStatusDistribution(context.Background(), "1y")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid period")
}

// TestStatsService_GetTrafficVolume_ValidBucket verifies bucketed bytes_sent.
func TestStatsService_GetTrafficVolume_ValidBucket(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	buckets, err := svc.GetTrafficVolume(context.Background(), "1h")
	require.NoError(t, err)
	// Just verify it returns without error and each bucket has a non-empty label.
	for _, b := range buckets {
		assert.NotEmpty(t, b.Bucket)
		assert.GreaterOrEqual(t, b.BytesSent, int64(0))
	}
}

// TestStatsService_GetTrafficVolume_InvalidBucket verifies rejection.
func TestStatsService_GetTrafficVolume_InvalidBucket(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	svc := NewStatsService(db)

	_, err := svc.GetTrafficVolume(context.Background(), "12h")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bucket")
}

// TestStatsService_GetTrafficVolume_6hBucket verifies the 6-hour bucket SQL path.
func TestStatsService_GetTrafficVolume_6hBucket(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	buckets, err := svc.GetTrafficVolume(context.Background(), "6h")
	require.NoError(t, err)
	for _, b := range buckets {
		assert.NotEmpty(t, b.Bucket)
		assert.GreaterOrEqual(t, b.BytesSent, int64(0))
	}
}

// TestStatsService_GetTrafficVolume_1dBucket verifies the 1-day bucket SQL path.
func TestStatsService_GetTrafficVolume_1dBucket(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	seedRequestLogs(t, db)
	svc := NewStatsService(db)

	buckets, err := svc.GetTrafficVolume(context.Background(), "1d")
	require.NoError(t, err)
	for _, b := range buckets {
		assert.NotEmpty(t, b.Bucket)
		assert.GreaterOrEqual(t, b.BytesSent, int64(0))
	}
}

// TestStatsService_GetSummary_DBError verifies that a closed DB returns a wrapped error.
func TestStatsService_GetSummary_DBError(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	svc := NewStatsService(db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = svc.GetSummary(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetSummary")
}

// TestStatsService_GetCertExpiry_ValidWithinDays verifies certs expiring soon are returned.
func TestStatsService_GetCertExpiry_ValidWithinDays(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	svc := NewStatsService(db)

	expiresSoon := time.Now().UTC().Add(5 * 24 * time.Hour)
	expiresLater := time.Now().UTC().Add(60 * 24 * time.Hour)

	certSoon := models.SSLCertificate{
		UUID:      "cert-soon",
		Name:      "soon.example.com",
		ExpiresAt: &expiresSoon,
	}
	certLater := models.SSLCertificate{
		UUID:      "cert-later",
		Name:      "later.example.com",
		ExpiresAt: &expiresLater,
	}
	require.NoError(t, db.Create(&certSoon).Error)
	require.NoError(t, db.Create(&certLater).Error)

	certID := certSoon.ID
	host := models.ProxyHost{
		UUID:          "host-cert-uuid",
		DomainNames:   "soon.example.com",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		CertificateID: &certID,
	}
	require.NoError(t, db.Create(&host).Error)

	results, err := svc.GetCertExpiry(context.Background(), 30)
	require.NoError(t, err)

	require.NotEmpty(t, results)
	found := false
	for _, r := range results {
		if r.HostID == host.UUID {
			found = true
			assert.Equal(t, "soon.example.com", r.Hostname)
			assert.InDelta(t, 5, r.DaysLeft, 1)
		}
	}
	assert.True(t, found, "expected to find cert-soon host in results")
}

// TestStatsService_GetCertExpiry_Boundary365 verifies upper boundary is accepted.
func TestStatsService_GetCertExpiry_Boundary365(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	svc := NewStatsService(db)

	_, err := svc.GetCertExpiry(context.Background(), 365)
	require.NoError(t, err)
}

// TestStatsService_GetCertExpiry_InvalidZero verifies withinDays=0 is rejected.
func TestStatsService_GetCertExpiry_InvalidZero(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	svc := NewStatsService(db)

	_, err := svc.GetCertExpiry(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "withinDays must be between 1 and 365")
}

// TestStatsService_GetCertExpiry_Invalid366 verifies withinDays=366 is rejected.
func TestStatsService_GetCertExpiry_Invalid366(t *testing.T) {
	t.Parallel()

	db := setupStatsServiceDB(t)
	svc := NewStatsService(db)

	_, err := svc.GetCertExpiry(context.Background(), 366)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "withinDays must be between 1 and 365")
}
