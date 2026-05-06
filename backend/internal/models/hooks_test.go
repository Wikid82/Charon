package models

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&NotificationTemplate{}, &UptimeHost{}, &UptimeNotificationEvent{}, &OrthrusAgent{}, &TunnelConfig{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func TestNotificationTemplate_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	tmpl := &NotificationTemplate{
		Name: "hook-test",
	}
	if err := db.Create(tmpl).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if tmpl.ID == "" {
		t.Fatalf("expected ID to be populated by BeforeCreate")
	}
}

func TestUptimeHost_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	h := &UptimeHost{
		Host: "127.0.0.1",
	}
	if err := db.Create(h).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if h.ID == "" {
		t.Fatalf("expected ID to be populated by BeforeCreate")
	}
	if h.Status != "pending" {
		t.Fatalf("expected default Status 'pending', got %q", h.Status)
	}
}

func TestUptimeNotificationEvent_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	e := &UptimeNotificationEvent{
		HostID:    "host-1",
		EventType: "down",
	}
	if err := db.Create(e).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if e.ID == "" {
		t.Fatalf("expected ID to be populated by BeforeCreate")
	}
}

func TestOrthrusAgent_BeforeCreate_SetsUUID(t *testing.T) {
	db := setupTestDB(t)
	agent := &OrthrusAgent{
		Name:        "hook-test-agent",
		AuthKeyHash: "somehash",
		Status:      OrthrusStatusPending,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if agent.UUID == "" {
		t.Fatalf("expected UUID to be populated by BeforeCreate")
	}
}

func TestOrthrusAgent_BeforeCreate_PreservesUUID(t *testing.T) {
	db := setupTestDB(t)
	agent := &OrthrusAgent{
		UUID:        "preset-uuid",
		Name:        "preset-agent",
		AuthKeyHash: "somehash",
		Status:      OrthrusStatusPending,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if agent.UUID != "preset-uuid" {
		t.Fatalf("expected UUID to remain 'preset-uuid', got %q", agent.UUID)
	}
}

func TestTunnelConfig_BeforeCreate_SetsUUID(t *testing.T) {
	db := setupTestDB(t)
	cfg := &TunnelConfig{
		Name:                 "hook-test-tunnel",
		Provider:             ProviderNetBird,
		EncryptedCredentials: "encrypted",
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if cfg.UUID == "" {
		t.Fatalf("expected UUID to be populated by BeforeCreate")
	}
}

func TestTunnelConfig_BeforeCreate_PreservesUUID(t *testing.T) {
	db := setupTestDB(t)
	cfg := &TunnelConfig{
		UUID:                 "preset-tunnel-uuid",
		Name:                 "preset-tunnel",
		Provider:             ProviderNetBird,
		EncryptedCredentials: "encrypted",
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if cfg.UUID != "preset-tunnel-uuid" {
		t.Fatalf("expected UUID to remain 'preset-tunnel-uuid', got %q", cfg.UUID)
	}
}
