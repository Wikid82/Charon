package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUptimeService_sendRecoveryNotification(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Notification{}, &models.NotificationProvider{}))

	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)

	monitor := models.UptimeMonitor{Name: "API Server", URL: "https://api.example.com"}

	svc.sendRecoveryNotification(monitor, "5m")

	var notifications []models.Notification
	require.NoError(t, db.Find(&notifications).Error)

	require.Len(t, notifications, 1)
	assert.Contains(t, notifications[0].Title, "API Server")
	assert.Contains(t, notifications[0].Message, "Downtime: 5m")
	assert.Equal(t, models.NotificationTypeSuccess, notifications[0].Type)
}
