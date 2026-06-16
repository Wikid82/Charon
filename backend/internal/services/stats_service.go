package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

const (
	maxTopHostsLimit = 50
	summaryCacheTTL  = 30 * time.Second
	withinDaysMin    = 1
	withinDaysMax    = 365
)

// validPeriods is the allowlist for period query parameters.
var validPeriods = map[string]time.Duration{
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

// validBuckets is the allowlist for traffic volume bucket parameters.
var validBuckets = map[string]time.Duration{
	"1h": time.Hour,
	"6h": 6 * time.Hour,
	"1d": 24 * time.Hour,
}

// summaryCache holds a cached StatsSummary with an expiry time.
type summaryCache struct {
	mu          sync.Mutex
	cachedValue StatsSummary
	expiresAt   time.Time
}

func (c *summaryCache) get() (StatsSummary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expiresAt) {
		return c.cachedValue, true
	}
	return StatsSummary{}, false
}

func (c *summaryCache) set(v StatsSummary, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedValue = v
	c.expiresAt = time.Now().Add(ttl)
}

// StatsService provides aggregated query results for the dashboard API handlers.
type StatsService struct {
	db    *gorm.DB
	cache summaryCache
}

// NewStatsService creates a StatsService backed by the provided GORM DB.
func NewStatsService(db *gorm.DB) *StatsService {
	return &StatsService{db: db}
}

// GetSummary returns request counts for the last 24h, 7d, and 30d.
// Results are cached for 30 seconds to avoid redundant aggregation queries.
func (s *StatsService) GetSummary(ctx context.Context) (StatsSummary, error) {
	if cached, ok := s.cache.get(); ok {
		return cached, nil
	}

	now := time.Now().UTC()

	var summary StatsSummary
	type countResult struct {
		Count int64
	}

	windows := []struct {
		since  time.Time
		target *int64
	}{
		{now.Add(-24 * time.Hour), &summary.RequestsLast24h},
		{now.Add(-7 * 24 * time.Hour), &summary.RequestsLast7d},
		{now.Add(-30 * 24 * time.Hour), &summary.RequestsLast30d},
	}

	for _, w := range windows {
		var res countResult
		if err := s.db.WithContext(ctx).
			Model(&models.RequestLog{}).
			Select("COUNT(*) AS count").
			Where("timestamp >= ?", w.since).
			Scan(&res).Error; err != nil {
			return StatsSummary{}, fmt.Errorf("GetSummary: %w", err)
		}
		*w.target = res.Count
	}

	s.cache.set(summary, summaryCacheTTL)
	return summary, nil
}

// GetTopHosts returns the top N hosts by request count over the given period.
// period must be one of "24h", "7d", "30d". limit is capped at 50.
func (s *StatsService) GetTopHosts(ctx context.Context, period string, limit int) ([]HostStat, error) {
	dur, ok := validPeriods[period]
	if !ok {
		return nil, fmt.Errorf("invalid period: %s", period)
	}
	if limit > maxTopHostsLimit {
		limit = maxTopHostsLimit
	}

	since := time.Now().UTC().Add(-dur)

	type joinResult struct {
		HostID      string
		DomainNames string
		Count       int64
	}

	var rows []joinResult
	if err := s.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select("request_logs.host_id AS host_id, proxy_hosts.domain_names AS domain_names, COUNT(*) AS count").
		Joins("LEFT JOIN proxy_hosts ON proxy_hosts.uuid = request_logs.host_id").
		Where("request_logs.timestamp >= ?", since).
		Group("request_logs.host_id, proxy_hosts.domain_names").
		Order("count DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetTopHosts: %w", err)
	}

	results := make([]HostStat, 0, len(rows))
	for _, r := range rows {
		hostname := r.DomainNames
		if hostname == "" {
			hostname = r.HostID
		}
		results = append(results, HostStat{
			HostID:   r.HostID,
			Hostname: hostname,
			Count:    r.Count,
		})
	}

	return results, nil
}

// GetStatusDistribution returns HTTP status code counts over the given period.
// period must be one of "24h", "7d", "30d".
func (s *StatsService) GetStatusDistribution(ctx context.Context, period string) ([]StatusStat, error) {
	dur, ok := validPeriods[period]
	if !ok {
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	since := time.Now().UTC().Add(-dur)

	var results []StatusStat
	if err := s.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select("status_code AS code, COUNT(*) AS count").
		Where("timestamp >= ?", since).
		Group("status_code").
		Order("count DESC").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("GetStatusDistribution: %w", err)
	}

	return results, nil
}

// GetTrafficVolume returns bytes sent bucketed by time interval.
// bucket must be one of "1h", "6h", "1d".
func (s *StatsService) GetTrafficVolume(ctx context.Context, bucket string) ([]TrafficBucket, error) {
	bucketDur, ok := validBuckets[bucket]
	if !ok {
		return nil, fmt.Errorf("invalid bucket: %s", bucket)
	}

	// Determine look-back window: show the last 30 buckets.
	lookback := bucketDur * 30
	since := time.Now().UTC().Add(-lookback)

	// SQLite strftime format derived from bucket size.
	var fmtStr string
	switch bucket {
	case "1h":
		fmtStr = "%Y-%m-%dT%H:00:00Z"
	case "6h":
		// Truncate to 6-hour blocks: hour rounded down to nearest 6.
		fmtStr = "%Y-%m-%dT%H:00:00Z" // will post-process below
	case "1d":
		fmtStr = "%Y-%m-%dT00:00:00Z"
	}

	type rawBucket struct {
		Bucket    string
		BytesSent int64
	}

	var raw []rawBucket

	if bucket == "6h" {
		// For 6h buckets, cast hour to integer, divide by 6, multiply by 6 to get the bucket start hour.
		if err := s.db.WithContext(ctx).
			Model(&models.RequestLog{}).
			Select("strftime('%Y-%m-%dT', timestamp) || printf('%02d', (CAST(strftime('%H', timestamp) AS INTEGER) / 6) * 6) || ':00:00Z' AS bucket, SUM(bytes_sent) AS bytes_sent").
			Where("timestamp >= ?", since).
			Group("bucket").
			Order("bucket ASC").
			Scan(&raw).Error; err != nil {
			return nil, fmt.Errorf("GetTrafficVolume: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).
			Model(&models.RequestLog{}).
			Select("strftime(?, timestamp) AS bucket, SUM(bytes_sent) AS bytes_sent", fmtStr).
			Where("timestamp >= ?", since).
			Group("bucket").
			Order("bucket ASC").
			Scan(&raw).Error; err != nil {
			return nil, fmt.Errorf("GetTrafficVolume: %w", err)
		}
	}

	results := make([]TrafficBucket, 0, len(raw))
	for _, r := range raw {
		results = append(results, TrafficBucket(r))
	}

	return results, nil
}

// GetCertExpiry returns certificates expiring within the given number of days.
// withinDays must be between 1 and 365 inclusive.
func (s *StatsService) GetCertExpiry(ctx context.Context, withinDays int) ([]CertExpiry, error) {
	if withinDays < withinDaysMin || withinDays > withinDaysMax {
		return nil, fmt.Errorf("withinDays must be between 1 and 365")
	}

	now := time.Now().UTC()
	deadline := now.Add(time.Duration(withinDays) * 24 * time.Hour)

	type joinResult struct {
		HostUUID    string
		DomainNames string
		ExpiresAt   *time.Time
	}

	var rows []joinResult
	if err := s.db.WithContext(ctx).
		Model(&models.ProxyHost{}).
		Select("proxy_hosts.uuid AS host_uuid, proxy_hosts.domain_names, ssl_certificates.expires_at").
		Joins("JOIN ssl_certificates ON ssl_certificates.id = proxy_hosts.certificate_id").
		Where("ssl_certificates.expires_at IS NOT NULL").
		Where("ssl_certificates.expires_at > ?", now).
		Where("ssl_certificates.expires_at <= ?", deadline).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetCertExpiry: %w", err)
	}

	results := make([]CertExpiry, 0, len(rows))
	for _, r := range rows {
		exp := time.Time{}
		if r.ExpiresAt != nil {
			exp = *r.ExpiresAt
		}
		daysLeft := int(time.Until(exp).Hours() / 24)
		results = append(results, CertExpiry{
			HostID:    r.HostUUID,
			Hostname:  r.DomainNames,
			ExpiresAt: exp,
			DaysLeft:  daysLeft,
		})
	}

	return results, nil
}
