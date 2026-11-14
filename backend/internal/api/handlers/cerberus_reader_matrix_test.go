package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// This file pins what the four Cerberus readers answer today. No reader changes here. A later change to any reader
// must show up as a diff in the golden file or as a failed scenario.
//
// The readers:
//   - proxy: caddy.CerberusEnabledForProxy, also reached at boot through services.ProxyCerberusCheck
//   - status: SecurityHandler.GetStatus (cerberus.enabled)
//   - middleware: cerberus.Cerberus.IsEnabled
//   - presets: CrowdsecHandler.isCerberusEnabled (presets, console and apply gate)

const (
	// readerKeyAbsent marks a switch key that is not stored. No stored value can equal it.
	readerKeyAbsent = "<absent>"
	// readerUpdateEnv set to "1" rewrites the golden file from the current answers instead of comparing.
	readerUpdateEnv  = "CHARON_UPDATE_READER_MATRIX"
	readerGoldenPath = "testdata/cerberus_reader_matrix.golden"
)

// readerSwitchEnvNames are the env overrides only the presets gate reads.
var readerSwitchEnvNames = []string{"FEATURE_CERBERUS_ENABLED", "CERBERUS_ENABLED"}

// readerStatics are the static configurations. "on" enables the suite statically. "off" is a non-empty config that no
// static heuristic reads as on. "zero" is the all-zero config, which the middleware reads as on by back-compat.
var readerStatics = map[string]config.SecurityConfig{
	"on":   {CerberusEnabled: true},
	"off":  {CrowdSecMode: "disabled"},
	"zero": {},
}

// readerStaticNames, readerRowNames and readerKeyValues span the matrix. Row values are "absent", "enabled" and
// "disabled". Key values cover every spelling the readers differ on, including blank and garbage.
var (
	readerStaticNames = []string{"on", "off", "zero"}
	readerRowNames    = []string{"absent", "enabled", "disabled"}
	readerKeyValues   = []string{readerKeyAbsent, "true", "false", "TRUE", "1", "yes", "", "garbage", " true", "true "}
	readerEnvValues   = []string{
		"FEATURE_CERBERUS_ENABLED=false",
		"FEATURE_CERBERUS_ENABLED=1",
		"FEATURE_CERBERUS_ENABLED=yes",
		"CERBERUS_ENABLED=false",
		"CERBERUS_ENABLED=1",
	}
)

// readerAnswers is what each reader says for one state.
type readerAnswers struct {
	proxy      bool
	status     bool
	middleware bool
	presets    bool
}

// readerState is one stored state of the matrix. env is "" or one NAME=value assignment.
type readerState struct {
	static  string
	row     string
	feature string
	legacy  string
	env     string
}

// readerScenario is a state reached through the real write paths from a seed row.
type readerScenario struct {
	name   string
	static string
	row    string
	steps  []func(t *testing.T, db *gorm.DB, h *SecurityHandler)
	want   readerAnswers
}

// seedReaderState opens a fresh database and stores the default row and the two switch keys. The row is "absent",
// "enabled" or "disabled". A key value of readerKeyAbsent leaves the key unstored.
func seedReaderState(t *testing.T, row, feature, legacy string) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	if row != "absent" {
		require.NoError(t, db.Create(&models.SecurityConfig{
			Name:    models.DefaultSecurityConfigName,
			Enabled: row == "enabled",
		}).Error)
	}
	keys := []struct{ key, value, category string }{
		{models.CerberusFeatureSettingKey, feature, "feature"},
		{models.CerberusLegacySettingKey, legacy, "security"},
	}
	for _, k := range keys {
		if k.value == readerKeyAbsent {
			continue
		}
		require.NoError(t, db.Create(&models.Setting{Key: k.key, Value: k.value, Category: k.category, Type: "bool"}).Error)
	}
	return db
}

// setReaderEnv clears both presets env overrides and sets the one named by assignment, if any. The cleanup restores
// the caller's environment.
func setReaderEnv(t *testing.T, assignment string) {
	t.Helper()
	for _, name := range readerSwitchEnvNames {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
	if assignment == "" {
		return
	}
	name, value, _ := strings.Cut(assignment, "=")
	t.Setenv(name, value)
}

// statusCerberusEnabled reads cerberus.enabled from the status endpoint.
func statusCerberusEnabled(t *testing.T, h *SecurityHandler) bool {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/security/status", http.NoBody)
	h.GetStatus(c)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Cerberus struct {
			Enabled bool `json:"enabled"`
		} `json:"cerberus"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Cerberus.Enabled
}

// readReaderAnswers records what each reader answers for the state in db. The boot proxy check must match the apply
// path, so a mismatch fails here.
func readReaderAnswers(t *testing.T, db *gorm.DB, static config.SecurityConfig) readerAnswers {
	t.Helper()
	var row *models.SecurityConfig
	if sc, err := models.FindDefaultSecurityConfig(db); err == nil {
		row = &sc
	}
	proxy := caddy.CerberusEnabledForProxy(db, static, row)
	require.Equal(t, proxy, services.ProxyCerberusCheck(db, static)(), "boot proxy check must match the apply path")

	h := NewSecurityHandler(static, db, nil)
	defer h.Close()
	return readerAnswers{
		proxy:      proxy,
		status:     statusCerberusEnabled(t, h),
		middleware: cerberus.New(static, db).IsEnabled(),
		presets:    (&CrowdsecHandler{DB: db}).isCerberusEnabled(),
	}
}

// serveAdminPost calls an admin-only toggle handler as an admin and requires 200.
func serveAdminPost(t *testing.T, handler gin.HandlerFunc) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/security/cerberus", http.NoBody)
	c.Set("role", string(models.RoleAdmin))
	handler(c)
	require.Equal(t, http.StatusOK, w.Code)
}

func stepEnableKey(t *testing.T, _ *gorm.DB, h *SecurityHandler) {
	serveAdminPost(t, h.EnableCerberus)
}

func stepDisableKey(t *testing.T, _ *gorm.DB, h *SecurityHandler) {
	serveAdminPost(t, h.DisableCerberus)
}

// stepFeatureFlag sends the System Settings Cerberus toggle through the settings batch path.
func stepFeatureFlag(value bool) func(t *testing.T, db *gorm.DB, h *SecurityHandler) {
	return func(t *testing.T, db *gorm.DB, _ *SecurityHandler) {
		t.Helper()
		body := fmt.Sprintf(`{%q: %t}`, models.CerberusFeatureSettingKey, value)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		NewFeatureFlagsHandler(db).UpdateFlags(c)
		require.Equal(t, http.StatusOK, w.Code)
	}
}

// stepBreakGlassDisable calls the localhost /security/disable, which writes the row and both Cerberus keys off.
func stepBreakGlassDisable(t *testing.T, _ *gorm.DB, h *SecurityHandler) {
	t.Helper()
	w := serveDisable(t, h, "127.0.0.1:5000", `{}`)
	require.Equal(t, http.StatusOK, w.Code)
}

// readerScenarios are the key-only and break-glass states, each with the answers recorded for it. Every answer is
// the current behavior, including the disagreements between readers.
var readerScenarios = []readerScenario{
	{
		name:   "fresh install row off no keys",
		static: "off",
		row:    "disabled",
		want:   readerAnswers{proxy: false, status: false, middleware: false, presets: true},
	},
	{
		name:   "fresh install then EnableCerberus key-only",
		static: "off",
		row:    "disabled",
		steps:  []func(*testing.T, *gorm.DB, *SecurityHandler){stepEnableKey},
		want:   readerAnswers{proxy: true, status: true, middleware: false, presets: true},
	},
	{
		name:   "fresh install then EnableCerberus then DisableCerberus",
		static: "off",
		row:    "disabled",
		steps: []func(*testing.T, *gorm.DB, *SecurityHandler){
			stepEnableKey, stepDisableKey,
		},
		want: readerAnswers{proxy: false, status: false, middleware: false, presets: false},
	},
	{
		name:   "fresh install then System Settings toggle on",
		static: "off",
		row:    "disabled",
		steps:  []func(*testing.T, *gorm.DB, *SecurityHandler){stepFeatureFlag(true)},
		want:   readerAnswers{proxy: true, status: true, middleware: false, presets: true},
	},
	{
		name:   "fresh install then System Settings toggle off",
		static: "off",
		row:    "disabled",
		steps:  []func(*testing.T, *gorm.DB, *SecurityHandler){stepFeatureFlag(false)},
		want:   readerAnswers{proxy: false, status: false, middleware: false, presets: false},
	},
	{
		name:   "row on no keys static off",
		static: "off",
		row:    "enabled",
		want:   readerAnswers{proxy: true, status: true, middleware: false, presets: true},
	},
	{
		name:   "row on then EnableCerberus key-only",
		static: "off",
		row:    "enabled",
		steps:  []func(*testing.T, *gorm.DB, *SecurityHandler){stepEnableKey},
		want:   readerAnswers{proxy: true, status: true, middleware: true, presets: true},
	},
	{
		// The break-glass disable switches the row and both keys off, so the stale keys no longer keep the proxy on.
		name:   "row on then EnableCerberus then break-glass disable",
		static: "off",
		row:    "enabled",
		steps: []func(*testing.T, *gorm.DB, *SecurityHandler){
			stepEnableKey, stepBreakGlassDisable,
		},
		want: readerAnswers{proxy: false, status: false, middleware: false, presets: false},
	},
	{
		name:   "row on no keys then break-glass disable",
		static: "off",
		row:    "enabled",
		steps:  []func(*testing.T, *gorm.DB, *SecurityHandler){stepBreakGlassDisable},
		want:   readerAnswers{proxy: false, status: false, middleware: false, presets: false},
	},
	{
		name:   "row on no keys static on",
		static: "on",
		row:    "enabled",
		want:   readerAnswers{proxy: true, status: true, middleware: true, presets: true},
	},
	{
		name:   "no row then EnableCerberus key-only",
		static: "off",
		row:    "absent",
		steps:  []func(*testing.T, *gorm.DB, *SecurityHandler){stepEnableKey},
		want:   readerAnswers{proxy: true, status: true, middleware: true, presets: true},
	},
	{
		name:   "no row then DisableCerberus",
		static: "off",
		row:    "absent",
		steps:  []func(*testing.T, *gorm.DB, *SecurityHandler){stepDisableKey},
		want:   readerAnswers{proxy: false, status: false, middleware: false, presets: false},
	},
	{
		name:   "no row no keys static on",
		static: "on",
		row:    "absent",
		want:   readerAnswers{proxy: true, status: true, middleware: true, presets: true},
	},
	{
		// The all-zero static config: the middleware reads it as on by back-compat, the proxy and status do not.
		name:   "no row no keys static zero",
		static: "zero",
		row:    "absent",
		want:   readerAnswers{proxy: false, status: false, middleware: true, presets: true},
	},
	{
		name:   "no row no keys static off",
		static: "off",
		row:    "absent",
		want:   readerAnswers{proxy: false, status: false, middleware: false, presets: true},
	},
}

// TestCerberusReaderScenarios pins the answers of the four readers for the key-only and break-glass states.
func TestCerberusReaderScenarios(t *testing.T) {
	for _, sc := range readerScenarios {
		t.Run(sc.name, func(t *testing.T) {
			setReaderEnv(t, "")
			db := seedReaderState(t, sc.row, readerKeyAbsent, readerKeyAbsent)
			static := readerStatics[sc.static]
			h := NewSecurityHandler(static, db, nil)
			defer h.Close()

			for _, step := range sc.steps {
				step(t, db, h)
			}
			require.Equal(t, sc.want, readReaderAnswers(t, db, static), "reader answers for %s", sc.name)
		})
	}
}

// readerMatrixStates spans the stored states: every static, row, feature key and legacy key combination with no env
// override, then every env override over the static and row states with both keys absent.
func readerMatrixStates() []readerState {
	var states []readerState
	for _, static := range readerStaticNames {
		for _, row := range readerRowNames {
			for _, feature := range readerKeyValues {
				for _, legacy := range readerKeyValues {
					states = append(states, readerState{static: static, row: row, feature: feature, legacy: legacy})
				}
			}
		}
	}
	for _, static := range readerStaticNames {
		for _, row := range readerRowNames {
			for _, env := range readerEnvValues {
				states = append(states, readerState{
					static: static, row: row, feature: readerKeyAbsent, legacy: readerKeyAbsent, env: env,
				})
			}
		}
	}
	return states
}

func renderReaderLine(st readerState, a readerAnswers) string {
	return fmt.Sprintf("static=%-4s row=%-8s feature=%-11q legacy=%-11q env=%-32q => proxy=%-5t status=%-5t middleware=%-5t presets=%-5t",
		st.static, st.row, st.feature, st.legacy, st.env, a.proxy, a.status, a.middleware, a.presets)
}

// readerPostDisablePrefix marks the golden rows recorded after a localhost break-glass Disable. They follow the stored
// state rows, so every row above them is the state before any Disable.
const readerPostDisablePrefix = "after-disable "

// readerPostDisableLines runs a localhost break-glass Disable on each stored state of the no-env grid and records the
// four answers afterwards.
func readerPostDisableLines(t *testing.T) []string {
	t.Helper()
	var lines []string
	for _, st := range readerMatrixStates() {
		if st.env != "" {
			continue
		}
		setReaderEnv(t, "")
		db := seedReaderState(t, st.row, st.feature, st.legacy)
		static := readerStatics[st.static]
		h := NewSecurityHandler(static, db, nil)
		w := serveDisable(t, h, "127.0.0.1:5000", `{}`)
		require.Equal(t, http.StatusOK, w.Code)
		h.Close()
		lines = append(lines, readerPostDisablePrefix+renderReaderLine(st, readReaderAnswers(t, db, static)))
	}
	return lines
}

// TestCerberusReaderMatrix records every stored state's answers, then the answers after a break-glass Disable of each
// state, and compares them with the golden file. Run with CHARON_UPDATE_READER_MATRIX=1 to rewrite the golden file
// after an intended change.
func TestCerberusReaderMatrix(t *testing.T) {
	var lines []string
	for _, st := range readerMatrixStates() {
		setReaderEnv(t, st.env)
		db := seedReaderState(t, st.row, st.feature, st.legacy)
		lines = append(lines, renderReaderLine(st, readReaderAnswers(t, db, readerStatics[st.static])))
	}
	lines = append(lines, readerPostDisableLines(t)...)
	got := strings.Join(lines, "\n") + "\n"

	if os.Getenv(readerUpdateEnv) == "1" {
		require.NoError(t, os.WriteFile(readerGoldenPath, []byte(got), 0o600))
		return
	}
	want, err := os.ReadFile(readerGoldenPath)
	require.NoError(t, err, "golden file missing; run with %s=1 to create it", readerUpdateEnv)
	require.Equal(t, string(want), got)
}
