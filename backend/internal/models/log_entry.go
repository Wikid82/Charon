package models

import (
	"errors"
	"strings"
)

// CaddyAccessLog represents a structured log entry from Caddy's JSON access logs.
type CaddyAccessLog struct {
	Level   string  `json:"level"`
	Ts      float64 `json:"ts"`
	Logger  string  `json:"logger"`
	Msg     string  `json:"msg"`
	Request struct {
		RemoteIP   string              `json:"remote_ip"`
		RemotePort string              `json:"remote_port"`
		ClientIP   string              `json:"client_ip"`
		Proto      string              `json:"proto"`
		Method     string              `json:"method"`
		Host       string              `json:"host"`
		URI        string              `json:"uri"`
		Headers    map[string][]string `json:"headers"`
		TLS        struct {
			Resumed     bool   `json:"resumed"`
			Version     int    `json:"version"`
			CipherSuite int    `json:"cipher_suite"`
			Proto       string `json:"proto"`
			ServerName  string `json:"server_name"`
		} `json:"tls"`
	} `json:"request"`
	BytesRead   int                 `json:"bytes_read"`
	UserID      string              `json:"user_id"`
	Duration    float64             `json:"duration"`
	Size        int                 `json:"size"`
	Status      int                 `json:"status"`
	RespHeaders map[string][]string `json:"resp_headers"`
}

// LogFilter defines criteria for filtering, sorting, and paginating logs.
type LogFilter struct {
	Search string `form:"search"`
	Host   string `form:"host"`
	Status string `form:"status"` // e.g., "200", "4xx", "5xx"
	Level  string `form:"level"`
	Limit  int    `form:"limit,default=50"`
	Offset int    `form:"offset,default=0"`
	Sort   string `form:"sort,default=desc"`  // direction: asc | desc
	SortBy string `form:"sort_by,default=ts"` // field: see ValidSortFields
}

// ValidSortFields is the allowlist for LogFilter.SortBy.
var ValidSortFields = map[string]bool{
	"ts":     true,
	"level":  true,
	"method": true,
	"uri":    true,
	"status": true,
}

// ErrInvalidSortBy is returned by Validate when SortBy is not allowlisted.
var ErrInvalidSortBy = errors.New("invalid sort_by: must be one of ts, level, method, uri, status")

// Validate normalizes and validates the filter in place: it clamps Limit to
// 1-500 (zero or negative values fall back to the default of 50), clamps
// Offset to >= 0, lowercases Sort/SortBy, defaults invalid Sort directions to
// "desc", and rejects SortBy values outside ValidSortFields.
func (f *LogFilter) Validate() error {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	f.Sort = strings.ToLower(f.Sort)
	if f.Sort != "asc" && f.Sort != "desc" {
		f.Sort = "desc"
	}

	f.SortBy = strings.ToLower(f.SortBy)
	if f.SortBy == "" {
		f.SortBy = "ts"
	}
	if !ValidSortFields[f.SortBy] {
		return ErrInvalidSortBy
	}
	return nil
}
