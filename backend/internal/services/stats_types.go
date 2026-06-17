// Package services provides business logic services for the application.
package services

import "time"

// StatsPushData holds a real-time stats snapshot for WebSocket broadcast.
type StatsPushData struct {
	RequestsLast24h int64        `json:"requests_last_24h"`
	RequestsLast7d  int64        `json:"requests_last_7d"`
	RequestsLast30d int64        `json:"requests_last_30d"`
	TopHosts        []HostStat   `json:"top_hosts"`
	StatusDist      []StatusStat `json:"status_distribution"`
}

// HostStat holds per-host request counts for the stats snapshot.
type HostStat struct {
	HostID   string `json:"host_id"`
	Hostname string `json:"hostname"`
	Count    int64  `json:"count"`
}

// StatusStat holds per-status-code request counts for the stats snapshot.
type StatusStat struct {
	Code  int   `json:"code"`
	Count int64 `json:"count"`
}

// StatsPushMessage wraps StatsPushData for WebSocket broadcast.
type StatsPushMessage struct {
	Type string        `json:"type"` // always "stats_update"
	Data StatsPushData `json:"data"`
}

// BroadcastHub is the interface StatsIngester uses to push to WebSocket clients.
type BroadcastHub interface {
	Broadcast(msg StatsPushMessage)
}

// StatsSummary holds aggregated request counts over multiple time windows.
type StatsSummary struct {
	RequestsLast24h int64 `json:"requests_last_24h"`
	RequestsLast7d  int64 `json:"requests_last_7d"`
	RequestsLast30d int64 `json:"requests_last_30d"`
}

// TrafficBucket holds bytes sent within a single time bucket.
type TrafficBucket struct {
	Bucket    string `json:"bucket"` // ISO 8601 truncated timestamp
	BytesSent int64  `json:"bytes_sent"`
}

// CertExpiry holds expiry metadata for a certificate attached to a proxy host.
type CertExpiry struct {
	HostID    string    `json:"host_id"`
	Hostname  string    `json:"hostname"`
	ExpiresAt time.Time `json:"expires_at"`
	DaysLeft  int       `json:"days_left"`
}
