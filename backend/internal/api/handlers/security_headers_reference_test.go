package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// TestDeleteProfile_AssignedWhileDeleting covers a host that takes the profile after the in-use count ran. The
// reference rule refuses the delete and the handler answers 409 without echoing the database error.
func TestDeleteProfile_AssignedWhileDeleting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.OpenMemoryDBWithForeignKeys(t)
	require.NoError(t, db.AutoMigrate(
		&models.SecurityHeaderProfile{}, &models.ProxyGroup{}, &models.AccessList{}, &models.SSLCertificate{},
		&models.DNSProvider{}, &models.DNSProviderCredential{}, &models.ProxyHost{}, &models.Location{},
		&models.RedirectionHost{},
	))
	profile := models.SecurityHeaderProfile{UUID: uuid.New().String(), Name: "Late"}
	require.NoError(t, db.Create(&profile).Error)
	profileID := profile.ID

	const callback = "test:assign_profile_before_delete"
	assigned := false
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "SecurityHeaderProfile" || assigned {
			return
		}
		assigned = true
		host := models.ProxyHost{UUID: uuid.New().String(), DomainNames: "late.example.com", ForwardHost: "localhost", ForwardPort: 80, SecurityHeaderProfileID: &profileID}
		if err := tx.Session(&gorm.Session{NewDB: true}).Create(&host).Error; err != nil {
			t.Errorf("assign host during delete: %v", err)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Delete().Remove(callback) })

	router := gin.New()
	registerSecurityHeadersRoutesForTest(router.Group("/"), NewSecurityHeadersHandler(db, nil))

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/security/headers/profiles/%d", profileID), http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.True(t, assigned)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "profile is in use by one or more proxy hosts")
}
