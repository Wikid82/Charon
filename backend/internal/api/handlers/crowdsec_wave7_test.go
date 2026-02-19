package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCrowdsecWave7_ReadAcquisitionConfig_ReadErrorOnDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	acqDir := filepath.Join(tmpDir, "acq")
	require.NoError(t, os.MkdirAll(acqDir, 0o750))

	_, err := readAcquisitionConfig(acqDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read acquisition config")
}

func TestCrowdsecWave7_Start_CreateSecurityConfigFailsOnReadOnlyDB(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "crowdsec-readonly.db")

	rwDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, rwDB.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	sqlDB, err := rwDB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	roDB, err := gorm.Open(sqlite.Open("file:"+dbPath+"?mode=ro"), &gorm.Config{})
	require.NoError(t, err)

	h := newTestCrowdsecHandler(t, roDB, &fakeExec{}, "/bin/false", t.TempDir())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)

	h.Start(c)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "Failed to persist configuration")
}

func TestCrowdsecWave7_EnsureBouncerRegistration_InvalidFileKeyReRegisters(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := tmpDir + "/bouncer_key"
	require.NoError(t, saveKeyToFile(keyPath, "invalid-file-key"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	db := setupCrowdDB(t)
	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	t.Setenv("CHARON_CROWDSEC_BOUNCER_KEY_PATH", keyPath)

	cfg := models.SecurityConfig{
		UUID:           uuid.New().String(),
		Name:           "default",
		CrowdSecAPIURL: server.URL,
	}
	require.NoError(t, db.Create(&cfg).Error)

	mockCmdExec := new(MockCommandExecutor)
	mockCmdExec.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return len(args) >= 2 && args[0] == "bouncers" && args[1] == "delete"
	})).Return([]byte("deleted"), nil)
	mockCmdExec.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return len(args) >= 2 && args[0] == "bouncers" && args[1] == "add"
	})).Return([]byte("new-file-key-1234567890"), nil)
	handler.CmdExec = mockCmdExec

	key, err := handler.ensureBouncerRegistration(context.Background())
	require.NoError(t, err)
	require.Equal(t, "new-file-key-1234567890", key)
	require.Equal(t, "new-file-key-1234567890", readKeyFromFile(keyPath))
	mockCmdExec.AssertExpectations(t)
}
