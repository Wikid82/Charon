package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestCheckExpiry_QueryFails covers lines 977-979: CheckExpiringCertificates fails.
func TestCheckExpiry_QueryFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.Notification{}, &models.NotificationProvider{}))

	// Drop ssl_certificates so CheckExpiringCertificates returns an error
	require.NoError(t, db.Exec("DROP TABLE ssl_certificates").Error)

	ns := NewNotificationService(db, nil)
	svc := NewCertificateService(t.TempDir(), db, nil)

	// Should not panic — logs the error and returns
	svc.checkExpiry(context.Background(), ns, 30)
}

// TestCheckExpiry_ExpiredCert_Success covers lines 981-998: expired cert notification success path.
func TestCheckExpiry_ExpiredCert_Success(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.Notification{}, &models.NotificationProvider{}))

	past := time.Now().Add(-48 * time.Hour)
	certUUID := uuid.New().String()
	require.NoError(t, db.Create(&models.SSLCertificate{
		UUID:      certUUID,
		Name:      "expired-cert",
		Provider:  "custom",
		Domains:   "expired.example.com",
		ExpiresAt: &past,
	}).Error)

	ns := NewNotificationService(db, nil)
	svc := NewCertificateService(t.TempDir(), db, nil)

	svc.checkExpiry(context.Background(), ns, 30)

	var notifications []models.Notification
	require.NoError(t, db.Find(&notifications).Error)
	assert.NotEmpty(t, notifications)
}

// TestCheckExpiry_ExpiringSoonCert_Success covers lines 999-1014: expiring-soon cert notification success path.
func TestCheckExpiry_ExpiringSoonCert_Success(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.Notification{}, &models.NotificationProvider{}))

	soon := time.Now().Add(7 * 24 * time.Hour)
	certUUID := uuid.New().String()
	require.NoError(t, db.Create(&models.SSLCertificate{
		UUID:      certUUID,
		Name:      "expiring-soon-cert",
		Provider:  "custom",
		Domains:   "soon.example.com",
		ExpiresAt: &soon,
	}).Error)

	ns := NewNotificationService(db, nil)
	svc := NewCertificateService(t.TempDir(), db, nil)

	svc.checkExpiry(context.Background(), ns, 30)

	var notifications []models.Notification
	require.NoError(t, db.Find(&notifications).Error)
	assert.NotEmpty(t, notifications)
}

// TestCheckExpiry_NotificationFails covers lines 991-992 and 1006-1007:
// Create() fails for both expired and expiring-soon certs.
func TestCheckExpiry_NotificationFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.Notification{}, &models.NotificationProvider{}))

	past := time.Now().Add(-48 * time.Hour)
	soon := time.Now().Add(7 * 24 * time.Hour)

	require.NoError(t, db.Create(&models.SSLCertificate{
		UUID:      uuid.New().String(),
		Name:      "expired-cert",
		Provider:  "custom",
		Domains:   "expired2.example.com",
		ExpiresAt: &past,
	}).Error)
	require.NoError(t, db.Create(&models.SSLCertificate{
		UUID:      uuid.New().String(),
		Name:      "soon-cert",
		Provider:  "custom",
		Domains:   "soon2.example.com",
		ExpiresAt: &soon,
	}).Error)

	// Drop notifications table so Create() fails
	require.NoError(t, db.Exec("DROP TABLE notifications").Error)

	ns := NewNotificationService(db, nil)
	svc := NewCertificateService(t.TempDir(), db, nil)

	// Should not panic — logs errors and continues
	svc.checkExpiry(context.Background(), ns, 30)
}

func TestUploadCertificate_KeyMismatch(t *testing.T) {
	cert1PEM, _ := generateTestCertAndKey(t, "cert1.example.com", time.Now().Add(24*time.Hour))
	_, key2PEM := generateTestCertAndKey(t, "cert2.example.com", time.Now().Add(24*time.Hour))

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	svc := NewCertificateService(t.TempDir(), db, nil)

	_, err = svc.UploadCertificate("mismatch-test", string(cert1PEM), string(key2PEM), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key validation failed")
}

func TestUploadCertificate_DBError(t *testing.T) {
	certPEM, keyPEM := generateTestCertAndKey(t, "db-err.example.com", time.Now().Add(24*time.Hour))

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	// No AutoMigrate → ssl_certificates table absent → db.Create fails

	svc := NewCertificateService(t.TempDir(), db, nil)

	_, err = svc.UploadCertificate("db-error-test", string(certPEM), string(keyPEM), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save certificate")
}

func TestGetCertificate_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	// No AutoMigrate → ssl_certificates table absent → First() returns error

	svc := NewCertificateService(t.TempDir(), db, nil)

	_, err = svc.GetCertificate(uuid.New().String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch certificate")
}

func TestUpdateCertificate_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	// No AutoMigrate → ssl_certificates table absent → First() returns non-ErrRecordNotFound error

	svc := NewCertificateService(t.TempDir(), db, nil)

	_, err = svc.UpdateCertificate(uuid.New().String(), "new-name")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch certificate")
}
