package services

import (
	"net"
	"testing"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUptimeTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	err = db.AutoMigrate(&models.Notification{}, &models.Setting{}, &models.ProxyHost{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	return db
}

func TestUptimeService_CheckHost(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db)
	us := NewUptimeService(db, ns)

	// Test Case 1: Host is UP
	// Start a listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	// Run check in a goroutine to accept connection if needed, but DialTimeout just needs handshake
	// Actually DialTimeout will succeed if listener is accepting.
	// We need to accept in a loop or just let it hang?
	// net.Dial will succeed as soon as handshake is done.
	// But we should probably accept to be clean.
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	up := us.CheckHost("127.0.0.1", port)
	assert.True(t, up, "Host should be UP")

	// Test Case 2: Host is DOWN
	// Use a port that is unlikely to be in use.
	// Or just close the listener and try again on same port (might be TIME_WAIT issues though)
	// Better to pick a random high port that nothing is listening on.
	// But finding a free port is tricky.
	// Let's just use a port we know is closed.
	// Or use the same port after closing listener.
	listener.Close()
	// Give it a moment
	time.Sleep(10 * time.Millisecond)

	down := us.CheckHost("127.0.0.1", port)
	assert.False(t, down, "Host should be DOWN")
}

func TestUptimeService_CheckAllHosts(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db)
	us := NewUptimeService(db, ns)

	// Create a dummy listener for a "UP" host
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()
	addr := listener.Addr().(*net.TCPAddr)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Seed ProxyHosts
	upHost := models.ProxyHost{
		UUID:        "uuid-1",
		DomainNames: "up.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: addr.Port,
		Enabled:     true,
	}
	db.Create(&upHost)

	downHost := models.ProxyHost{
		UUID:        "uuid-2",
		DomainNames: "down.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 54321, // Assuming this is closed
		Enabled:     true,
	}
	db.Create(&downHost)

	disabledHost := models.ProxyHost{
		UUID:        "uuid-3",
		DomainNames: "disabled.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 54322,
		Enabled:     false,
	}
	// Force Enabled=false by using map or Select
	db.Create(&disabledHost)
	db.Model(&disabledHost).Update("Enabled", false)

	// Run CheckAllHosts
	us.CheckAllHosts()

	// Verify Notifications
	var notifications []models.Notification
	db.Find(&notifications)

	for _, n := range notifications {
		t.Logf("Notification: %s - %s", n.Title, n.Message)
	}

	// We expect 1 notification for the downHost.
	// upHost is UP -> no notification
	// disabledHost is DISABLED -> no check -> no notification
	assert.Equal(t, 1, len(notifications), "Should have 1 notification")
	if len(notifications) > 0 {
		assert.Contains(t, notifications[0].Message, "down.example.com", "Notification should mention the down host")
		assert.Equal(t, models.NotificationTypeError, notifications[0].Type)
	}
}
