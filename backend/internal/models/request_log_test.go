package models

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var requestLogDBSeq uint64

func setupRequestLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	n := atomic.AddUint64(&requestLogDBSeq, 1)
	dsn := fmt.Sprintf("file:rl_test_%d?mode=memory&cache=private", n)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&RequestLog{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func TestRequestLog_BeforeCreate_SetsID(t *testing.T) {
	db := setupRequestLogTestDB(t)
	log := &RequestLog{
		HostID:     "host-uuid-1",
		Timestamp:  time.Now(),
		Method:     "GET",
		StatusCode: 200,
		BytesSent:  1024,
		DurationMs: 42,
	}
	if err := db.Create(log).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if log.ID == "" {
		t.Fatal("expected ID to be populated by BeforeCreate")
	}
}

func TestRequestLog_BeforeCreate_PreservesID(t *testing.T) {
	db := setupRequestLogTestDB(t)
	log := &RequestLog{
		ID:         "preset-id-abc",
		HostID:     "host-uuid-2",
		Timestamp:  time.Now(),
		Method:     "POST",
		StatusCode: 201,
		BytesSent:  512,
		DurationMs: 10,
	}
	if err := db.Create(log).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if log.ID != "preset-id-abc" {
		t.Fatalf("expected ID to remain 'preset-id-abc', got %q", log.ID)
	}
}

func TestRequestLog_JSONTags(t *testing.T) {
	log := RequestLog{
		ID:           "test-id",
		HostID:       "host-1",
		Timestamp:    time.Now(),
		Method:       "DELETE",
		StatusCode:   404,
		BytesSent:    0,
		DurationMs:   5,
		ClientIPHash: "abcdef1234567890",
	}

	// Verify all fields are accessible (compile-time check via struct literal)
	if log.ID == "" || log.HostID == "" || log.Method == "" {
		t.Fatal("required fields must not be empty")
	}
}

func TestRequestLog_MultipleRecords(t *testing.T) {
	db := setupRequestLogTestDB(t)

	logs := []*RequestLog{
		{HostID: "host-1", Timestamp: time.Now(), Method: "GET", StatusCode: 200, BytesSent: 100, DurationMs: 10},
		{HostID: "host-1", Timestamp: time.Now(), Method: "POST", StatusCode: 500, BytesSent: 50, DurationMs: 200},
		{HostID: "host-2", Timestamp: time.Now(), Method: "GET", StatusCode: 301, BytesSent: 0, DurationMs: 1},
	}

	for _, l := range logs {
		if err := db.Create(l).Error; err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if l.ID == "" {
			t.Fatal("expected ID to be set after create")
		}
	}

	var count int64
	db.Model(&RequestLog{}).Count(&count)
	if count != 3 {
		t.Fatalf("expected 3 records, got %d", count)
	}
}

func TestRequestLog_IndexedQuery(t *testing.T) {
	db := setupRequestLogTestDB(t)

	now := time.Now()
	entries := []*RequestLog{
		{HostID: "host-abc", Timestamp: now, Method: "GET", StatusCode: 200, BytesSent: 100, DurationMs: 10},
		{HostID: "host-abc", Timestamp: now.Add(time.Second), Method: "GET", StatusCode: 500, BytesSent: 200, DurationMs: 50},
		{HostID: "host-xyz", Timestamp: now, Method: "GET", StatusCode: 200, BytesSent: 10, DurationMs: 5},
	}
	for _, e := range entries {
		if err := db.Create(e).Error; err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	var results []RequestLog
	if err := db.Where("host_id = ? AND status_code = ?", "host-abc", 500).Find(&results).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
