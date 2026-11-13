package routes

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// syncBuffer is a goroutine-safe log sink. Register starts background workers
// that may log while the test is reading the output.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// bootstrapForSetupCodeLog registers the routes against an empty database with the
// given CHARON_SETUP_TOKEN value and returns the router plus everything logged
// during registration.
func bootstrapForSetupCodeLog(t *testing.T, setupToken string) (router *gin.Engine, logs string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var sink syncBuffer
	logger.Init(false, &sink)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	dsn := "file:" + filepath.Join(t.TempDir(), "setup_code_log.db") + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	router = gin.New()
	cfg := config.Config{JWTSecret: "test-secret", SetupToken: setupToken}
	require.NoError(t, Register(context.Background(), router, db, cfg))
	return router, sink.String()
}

// TestRegister_PresetSetupTokenIsNeverLogged verifies that an operator-preset
// CHARON_SETUP_TOKEN never reaches the log, and that exactly one neutral line
// records where the code came from.
func TestRegister_PresetSetupTokenIsNeverLogged(t *testing.T) {
	_, logs := bootstrapForSetupCodeLog(t, routesTestSetupCode)

	assert.NotContains(t, logs, routesTestSetupCode, "the preset must never appear in the log output")
	assert.NotContains(t, logs, "routes-test-setup-code", "no fragment of the preset may be logged")
	assert.Equal(t, 0, strings.Count(logs, "Setup code:"), "a preset must not use the generated-code line")
	assert.Equal(t, 1, strings.Count(logs, "Setup code taken from CHARON_SETUP_TOKEN"),
		"exactly one neutral line must be logged for a preset")
}

// generatedSetupCodePattern captures the code from the generated-code log line.
var generatedSetupCodePattern = regexp.MustCompile(`Setup code: ([A-Z2-7]+)\.`)

// TestRegister_GeneratedSetupCodeIsLoggedOnce verifies that a generated code is
// logged exactly once and that the logged value is the one that creates the first
// account.
func TestRegister_GeneratedSetupCodeIsLoggedOnce(t *testing.T) {
	router, logs := bootstrapForSetupCodeLog(t, "")

	assert.Equal(t, 1, strings.Count(logs, "Setup code:"), "a generated code must be logged exactly once")
	assert.Equal(t, 0, strings.Count(logs, "CHARON_SETUP_TOKEN"), "no preset line for a generated code")

	match := generatedSetupCodePattern.FindStringSubmatch(logs)
	require.Len(t, match, 2, "the generated code must be parseable from the log")

	body := `{"name":"First Admin","email":"admin@example.com","password":"adminpassword123","setup_code":"` + match[1] + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code, "the logged code must create the first account")
}
