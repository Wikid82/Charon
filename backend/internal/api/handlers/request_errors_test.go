package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

const storageDetail = "storage detail: disk I/O error"

// failTable makes every query, insert, update and delete on table fail with storageDetail.
func failTable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	fail := func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(errors.New(storageDetail))
		}
	}
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_query_"+table, fail))
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_create_"+table, fail))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:fail_update_"+table, fail))
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:fail_delete_"+table, fail))
}

// respondInternalError answers the public text only and logs the sanitized detail with the request logger.
func TestRespondInternalErrorAnswersThePublicTextOnly(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	log := logrus.New()
	log.SetOutput(&logs)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	c.Set("logger", logrus.NewEntry(log))

	respondInternalError(c, "Could not do the thing.", errors.New("table \"secrets\" locked\nsecond line"))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"Could not do the thing."}`, w.Body.String())
	assert.Contains(t, logs.String(), "locked", "the detail is logged")
	assert.NotContains(t, logs.String(), "\nsecond line", "the logged detail is sanitized")
}

// Every credential endpoint answers a storage failure with a fixed text, never with the storage detail. Lookups of
// the credential run first, so an endpoint whose lookup fails answers the lookup text.
func TestCredentialEndpointsAnswerStorageFailuresWithFixedText(t *testing.T) {
	t.Parallel()
	body := `{"label":"x","zone_filter":"example.com","credentials":{"api_token":"t"},"propagation_timeout":180,"polling_interval":10,"enabled":true}`

	check := func(t *testing.T, router *gin.Engine, method, path, allowed1, allowed2 string) {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
		assert.NotContains(t, w.Body.String(), storageDetail)
		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		var answer map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
		assert.Contains(t, []string{allowed1, allowed2}, answer["error"])
	}

	t.Run("reads and creates", func(t *testing.T) {
		t.Parallel()
		router, db, provider := setupCredentialStorageFixture(t)
		base := "/api/v1/dns-providers/" + strconv.FormatUint(uint64(provider.ID), 10)
		failTable(t, db, "dns_provider_credentials")

		check(t, router, http.MethodGet, base+"/credentials", "Could not load the credentials.", "")
		check(t, router, http.MethodPost, base+"/credentials", "Could not create the credential.", "Could not look up the credential.")
		check(t, router, http.MethodGet, base+"/credentials/1", "Could not load the credential.", "Could not look up the credential.")
		check(t, router, http.MethodPost, base+"/credentials/1/test", "Could not test the credential.", "Could not look up the credential.")
	})
	t.Run("updates and deletes", func(t *testing.T) {
		t.Parallel()
		router, db, provider := setupCredentialStorageFixture(t)
		seed := models.DNSProviderCredential{UUID: uuid.NewString(), DNSProviderID: provider.ID, Label: "seed", Enabled: true, CredentialsEncrypted: "x"}
		require.NoError(t, db.Create(&seed).Error)
		base := "/api/v1/dns-providers/" + strconv.FormatUint(uint64(provider.ID), 10) + "/credentials/" + strconv.FormatUint(uint64(seed.ID), 10)
		// Writes fail; the lookups before them keep working.
		fail := func(tx *gorm.DB) {
			if tx.Statement.Table == "dns_provider_credentials" {
				_ = tx.AddError(errors.New(storageDetail))
			}
		}
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:fail_update", fail))
		require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:fail_delete", fail))

		check(t, router, http.MethodPut, base, "Could not update the credential.", "Could not look up the credential.")
		check(t, router, http.MethodDelete, base, "Could not delete the credential.", "Could not look up the credential.")
	})
}

// The host list answers a storage failure with its fixed text and no detail.
func TestProxyHostListStorageFailureAnswersFixedText(t *testing.T) {
	t.Parallel()
	db := openHostRestoreDB(t)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})
	failTable(t, db, "proxy_hosts")

	w := f.do(t, http.MethodGet, "/api/v1/proxy-hosts", nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"Could not load the proxy hosts."}`, w.Body.String())
}

// setupCredentialStorageFixture serves the DNS credential routes over a database with one provider.
func setupCredentialStorageFixture(t *testing.T) (*gin.Engine, *gorm.DB, models.DNSProvider) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{}, &models.SecurityAudit{}))

	encryptor, err := crypto.NewEncryptionService("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	require.NoError(t, err)
	provider := models.DNSProvider{
		UUID:                uuid.NewString(),
		Name:                "Storage Provider",
		ProviderType:        "cloudflare",
		Enabled:             true,
		UseMultiCredentials: true,
		PropagationTimeout:  120,
		PollingInterval:     5,
	}
	require.NoError(t, db.Create(&provider).Error)

	h := NewCredentialHandler(services.NewCredentialService(db, encryptor), services.NewDNSProviderService(db, encryptor))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/dns-providers/:id/credentials", h.List)
	router.POST("/api/v1/dns-providers/:id/credentials", h.Create)
	router.GET("/api/v1/dns-providers/:id/credentials/:cred_id", h.Get)
	router.PUT("/api/v1/dns-providers/:id/credentials/:cred_id", h.Update)
	router.DELETE("/api/v1/dns-providers/:id/credentials/:cred_id", h.Delete)
	router.POST("/api/v1/dns-providers/:id/credentials/:cred_id/test", h.Test)
	router.POST("/api/v1/dns-providers/:id/enable-multi-credentials", h.EnableMultiCredentials)
	return router, db, provider
}
