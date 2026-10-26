package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// fullUpdateTestRow returns a default row with every updatable column set to a non-zero value. The break-glass
// hash is set too, so a write that reaches it shows up in the comparison.
func fullUpdateTestRow() models.SecurityConfig {
	return models.SecurityConfig{ //nolint:gosec // G101: test fixture, not a credential
		Name:                models.DefaultSecurityConfigName,
		Enabled:             true,
		AdminWhitelist:      "192.0.2.0/24",
		BreakGlassHash:      "stored-break-glass-hash",
		CrowdSecMode:        "local",
		CrowdSecAPIURL:      "http://crowdsec.example:8080",
		WAFMode:             "monitor",
		WAFRulesSource:      "owasp-crs",
		WAFLearning:         true,
		WAFParanoiaLevel:    2,
		WAFExclusions:       `[{"rule_id":942100}]`,
		RateLimitMode:       "enabled",
		RateLimitEnable:     true,
		RateLimitBurst:      4,
		RateLimitRequests:   50,
		RateLimitWindowSec:  7,
		RateLimitBypassList: "198.51.100.0/24",
	}
}

// seedUpdateTestRow stores row and returns the stored copy.
func seedUpdateTestRow(t *testing.T, db *gorm.DB, row models.SecurityConfig) models.SecurityConfig {
	t.Helper()
	require.NoError(t, db.Create(&row).Error)
	return loadUpdateTestRow(t, db)
}

func loadUpdateTestRow(t *testing.T, db *gorm.DB) models.SecurityConfig {
	t.Helper()
	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	return row
}

// decodeUpdateTestBody decodes a request body the way the handler does, so the tests cover the real decode.
func decodeUpdateTestBody(t *testing.T, body string) SecurityConfigUpdate {
	t.Helper()
	var update SecurityConfigUpdate
	require.NoError(t, json.Unmarshal([]byte(body), &update))
	return update
}

// setUpdateTestField sets one named field of row to value.
func setUpdateTestField(t *testing.T, row *models.SecurityConfig, field string, value any) {
	t.Helper()
	f := reflect.ValueOf(row).Elem().FieldByName(field)
	require.True(t, f.IsValid(), "unknown field %s", field)
	f.Set(reflect.ValueOf(value))
}

// requireUpdateTestRowEqual compares every column of want and got, except the update timestamp.
func requireUpdateTestRowEqual(t *testing.T, want, got models.SecurityConfig) {
	t.Helper()
	w := reflect.ValueOf(want)
	g := reflect.ValueOf(got)
	typ := w.Type()
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if name == "UpdatedAt" {
			continue
		}
		assert.Equal(t, w.Field(i).Interface(), g.Field(i).Interface(), "column %s", name)
	}
}

// updateFieldCase names one request key, the model field it stores, and a non-zero and a zero value of that field.
var updateFieldCases = []struct {
	key     string
	field   string
	nonZero any
	zero    any
}{
	{"enabled", "Enabled", true, false},
	{"admin_whitelist", "AdminWhitelist", "203.0.113.0/24", ""},
	{"crowdsec_api_url", "CrowdSecAPIURL", "http://crowdsec.example:9090", ""},
	{"waf_mode", "WAFMode", "block", ""},
	{"waf_rules_source", "WAFRulesSource", "custom-rules", ""},
	{"waf_learning", "WAFLearning", true, false},
	{"waf_paranoia_level", "WAFParanoiaLevel", 3, 0},
	{"waf_exclusions", "WAFExclusions", "[]", ""},
	{"rate_limit_burst", "RateLimitBurst", 9, 0},
	{"rate_limit_requests", "RateLimitRequests", 90, 0},
	{"rate_limit_window_sec", "RateLimitWindowSec", 11, 0},
	{"rate_limit_bypass_list", "RateLimitBypassList", "198.51.100.7/32", ""},
}

// Each case runs three ways against the fully populated row: a new non-zero value is applied, an explicit zero is
// applied, and an absent or null key keeps the stored value. The first run starts from the zero value of the field.
func TestApplyUpdate_AbsentVersusExplicitZero(t *testing.T) {
	for _, c := range updateFieldCases {
		c := c
		t.Run(c.key+" new value is applied", func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			seeded := fullUpdateTestRow()
			setUpdateTestField(t, &seeded, c.field, c.zero)
			before := seedUpdateTestRow(t, db, seeded)

			body := fmt.Sprintf(`{%q:%s}`, c.key, mustJSONLiteral(t, c.nonZero))
			got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, body))
			require.NoError(t, err)

			want := before
			setUpdateTestField(t, &want, c.field, c.nonZero)
			requireUpdateTestRowEqual(t, want, *got)
			requireUpdateTestRowEqual(t, want, loadUpdateTestRow(t, db))
		})

		t.Run(c.key+" explicit zero is applied", func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			before := seedUpdateTestRow(t, db, fullUpdateTestRow())

			body := fmt.Sprintf(`{%q:%s}`, c.key, mustJSONLiteral(t, c.zero))
			got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, body))
			require.NoError(t, err)

			want := before
			setUpdateTestField(t, &want, c.field, c.zero)
			requireUpdateTestRowEqual(t, want, *got)
			requireUpdateTestRowEqual(t, want, loadUpdateTestRow(t, db))
		})

		for _, absent := range []struct{ name, body string }{
			{"absent key keeps the stored value", `{"name":"default"}`},
			{"null key keeps the stored value", fmt.Sprintf(`{%q:null}`, c.key)},
		} {
			t.Run(c.key+" "+absent.name, func(t *testing.T) {
				db := setupSecurityTestDB(t)
				svc := newTestSecurityService(t, db)
				before := seedUpdateTestRow(t, db, fullUpdateTestRow())

				got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, absent.body))
				require.NoError(t, err)

				requireUpdateTestRowEqual(t, before, *got)
				requireUpdateTestRowEqual(t, before, loadUpdateTestRow(t, db))
			})
		}
	}
}

// mustJSONLiteral encodes value as a JSON literal for a request body.
func mustJSONLiteral(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}

// The rate-limit mode rule: each row is one combination of the request's enable flag and mode keys.
func TestApplyUpdate_RateLimitModeRule(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		seedEnable bool
		seedMode   string
		wantEnable bool
		wantMode   string
	}{
		{"enable true gives enabled", `{"rate_limit_enable":true,"rate_limit_mode":"disabled"}`, false, "disabled", true, "enabled"},
		{"enable false keeps a non-empty mode", `{"rate_limit_enable":false,"rate_limit_mode":"custom"}`, true, "enabled", false, "custom"},
		{"enable false without a mode gives disabled", `{"rate_limit_enable":false}`, true, "enabled", false, "disabled"},
		{"enable false with an empty mode gives disabled", `{"rate_limit_enable":false,"rate_limit_mode":""}`, true, "enabled", false, "disabled"},
		{"enable false with a null mode gives disabled", `{"rate_limit_enable":false,"rate_limit_mode":null}`, true, "enabled", false, "disabled"},
		{"mode only is applied and enable is kept", `{"rate_limit_mode":"custom"}`, true, "enabled", true, "custom"},
		{"empty mode only is stored as sent and enable is kept", `{"rate_limit_mode":""}`, true, "enabled", true, ""},
		{"three numbers leave enable and mode untouched", `{"rate_limit_requests":20,"rate_limit_burst":4,"rate_limit_window_sec":30}`, true, "enabled", true, "enabled"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			seeded := fullUpdateTestRow()
			seeded.RateLimitEnable = c.seedEnable
			seeded.RateLimitMode = c.seedMode
			seedUpdateTestRow(t, db, seeded)

			got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, c.body))
			require.NoError(t, err)

			assert.Equal(t, c.wantEnable, got.RateLimitEnable)
			assert.Equal(t, c.wantMode, got.RateLimitMode)
			stored := loadUpdateTestRow(t, db)
			assert.Equal(t, c.wantEnable, stored.RateLimitEnable)
			assert.Equal(t, c.wantMode, stored.RateLimitMode)
		})
	}
}

// A body with no updatable key must not reach an UPDATE: updated_at stays put and no row is written.
func TestApplyUpdate_BodyWithoutUpdatableKeyIssuesNoUpdate(t *testing.T) {
	for _, body := range []string{`{}`, `{"name":"default"}`, `{"crowdsec_mode":"local"}`} {
		body := body
		t.Run(body, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			before := seedUpdateTestRow(t, db, fullUpdateTestRow())

			rowUpdates := 0
			require.NoError(t, db.Callback().Update().After("gorm:update").Register("test:count_row_updates", func(tx *gorm.DB) {
				if tx.Statement.Table == "security_configs" && tx.RowsAffected > 0 {
					rowUpdates++
				}
			}))

			got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, body))
			require.NoError(t, err)

			assert.Equal(t, 0, rowUpdates, "no row is written")
			assert.Equal(t, before.UpdatedAt, got.UpdatedAt, "updated_at is not bumped")
			assert.Equal(t, before, loadUpdateTestRow(t, db))
		})
	}
}

// A crowdsec_mode key is validated but never stored, even when the request is the only thing sent.
func TestApplyUpdate_CrowdSecModeIsValidatedAndNeverStored(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seeded := fullUpdateTestRow()
	seeded.CrowdSecMode = "disabled"
	seedUpdateTestRow(t, db, seeded)

	got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"crowdsec_mode":"local","waf_mode":"block"}`))
	require.NoError(t, err)

	assert.Equal(t, "disabled", got.CrowdSecMode, "the stored mode is kept")
	assert.Equal(t, "block", got.WAFMode, "the other present fields are saved")
	assert.Equal(t, "disabled", loadUpdateTestRow(t, db).CrowdSecMode)
}

// Invalid values are refused before any write: the stored row is unchanged afterwards.
func TestApplyUpdate_InvalidValuesWriteNothing(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr error
		wantMsg string
	}{
		{"invalid allow-list entry", `{"admin_whitelist":"not-a-cidr","waf_mode":"block"}`, ErrInvalidAdminCIDR, ""},
		{"external crowdsec mode", `{"crowdsec_mode":"external","waf_mode":"block"}`, nil, "invalid crowdsec mode: external"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			before := seedUpdateTestRow(t, db, fullUpdateTestRow())

			got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, c.body))
			require.Error(t, err)
			assert.Nil(t, got)
			if c.wantErr != nil {
				assert.True(t, errors.Is(err, c.wantErr), "got %v", err)
			}
			if c.wantMsg != "" {
				assert.Contains(t, err.Error(), c.wantMsg)
			}
			assert.Equal(t, before, loadUpdateTestRow(t, db))
		})
	}
}

// An explicit empty allow-list clears the stored value and leaves every other column alone.
func TestApplyUpdate_ExplicitEmptyAllowListClearsIt(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	before := seedUpdateTestRow(t, db, fullUpdateTestRow())

	_, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"admin_whitelist":""}`))
	require.NoError(t, err)

	want := before
	want.AdminWhitelist = ""
	requireUpdateTestRowEqual(t, want, loadUpdateTestRow(t, db))
}

// A create path builds the row from the model defaults and the request's present fields.
func TestApplyUpdate_CreatesDefaultRowWhenNoneExists(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"admin_whitelist":"192.0.2.0/24"}`))
	require.NoError(t, err)

	assert.NotEmpty(t, got.UUID, "the UUID is generated server-side")
	assert.Equal(t, models.DefaultSecurityConfigName, got.Name)
	assert.Equal(t, "192.0.2.0/24", got.AdminWhitelist)
	assert.Equal(t, 1, got.WAFParanoiaLevel, "the paranoia level defaults to 1")
	assert.Equal(t, "disabled", got.RateLimitMode, "the rate-limit mode defaults to disabled")
	assert.Equal(t, got.UUID, loadUpdateTestRow(t, db).UUID)
}

// On the create path an explicit zero paranoia level is stored as the column default (1); on the update path
// the explicit zero is written as sent.
func TestApplyUpdate_ParanoiaLevelZeroOnCreateIsDefaulted(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	created, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_paranoia_level":0}`))
	require.NoError(t, err)
	assert.Equal(t, 1, created.WAFParanoiaLevel)

	updated, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_paranoia_level":0}`))
	require.NoError(t, err)
	assert.Equal(t, 0, updated.WAFParanoiaLevel)
	assert.Equal(t, 0, loadUpdateTestRow(t, db).WAFParanoiaLevel)
}

// The create path honors the rate-limit rule on top of the disabled default.
func TestApplyUpdate_CreatePathAppliesRateLimitRule(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"rate_limit_enable":true}`))
	require.NoError(t, err)
	assert.True(t, got.RateLimitEnable)
	assert.Equal(t, "enabled", got.RateLimitMode)
}

// Two callers that update different columns at the same time both keep their change, every round, with no deadlock.
func TestApplyUpdate_ConcurrentCallersKeepEachColumn(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedUpdateTestRow(t, db, fullUpdateTestRow())

	const rounds = 25
	var (
		mu       sync.Mutex
		failures []string
	)
	fail := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		failures = append(failures, msg)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 1; i <= rounds; i++ {
			requests := 1000 + i
			mode := fmt.Sprintf("mode-%d", i)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				if _, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, SecurityConfigUpdate{RateLimitRequests: &requests}); err != nil {
					fail(fmt.Sprintf("round %d rate limit update: %v", i, err))
				}
			}()
			go func() {
				defer wg.Done()
				if _, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, SecurityConfigUpdate{WAFMode: &mode}); err != nil {
					fail(fmt.Sprintf("round %d waf update: %v", i, err))
				}
			}()
			wg.Wait()

			var stored models.SecurityConfig
			if err := db.Where("name = ?", models.DefaultSecurityConfigName).First(&stored).Error; err != nil {
				fail(fmt.Sprintf("round %d read: %v", i, err))
				continue
			}
			if stored.RateLimitRequests != requests || stored.WAFMode != mode {
				fail(fmt.Sprintf("round %d lost a column: requests=%d waf=%q", i, stored.RateLimitRequests, stored.WAFMode))
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(45 * time.Second):
		t.Fatal("concurrent updates did not finish; a query may be running outside the write transaction")
	}
	assert.Empty(t, failures)
}

// Storage failures at each step of the write are returned to the caller, and a failed write leaves the row as it was.
// Each injected failure fires only while the call under test runs, so the verification reads are not affected.
func TestApplyUpdate_StorageFailuresAreReturned(t *testing.T) {
	injected := errors.New("injected storage failure")
	cases := []struct {
		name    string
		seeded  bool
		hook    func(db *gorm.DB, armed *bool) error
		wantMsg string
	}{
		{
			name:   "lookup fails",
			seeded: true,
			hook: func(db *gorm.DB, armed *bool) error {
				return db.Callback().Query().Before("gorm:query").Register("test:fail_lookup", func(tx *gorm.DB) {
					if *armed && tx.Statement.Table == "security_configs" {
						tx.Error = injected
					}
				})
			},
			wantMsg: "find security config",
		},
		{
			name:   "create fails",
			seeded: false,
			hook: func(db *gorm.DB, armed *bool) error {
				return db.Callback().Create().Before("gorm:create").Register("test:fail_create", func(tx *gorm.DB) {
					if *armed && tx.Statement.Table == "security_configs" {
						tx.Error = injected
					}
				})
			},
			wantMsg: "create security config",
		},
		{
			name:   "update fails",
			seeded: true,
			hook: func(db *gorm.DB, armed *bool) error {
				return db.Callback().Update().Before("gorm:update").Register("test:fail_update", func(tx *gorm.DB) {
					// The write lock issues an UPDATE without a column list; only the field update carries one.
					if *armed && tx.Statement.Table == "security_configs" && len(tx.Statement.Selects) > 0 {
						tx.Error = injected
					}
				})
			},
			wantMsg: "update security config",
		},
		{
			name:   "read-back fails",
			seeded: true,
			hook: func(db *gorm.DB, armed *bool) error {
				// The second lookup of the call is the read-back after the write.
				queries := 0
				return db.Callback().Query().Before("gorm:query").Register("test:fail_read_back", func(tx *gorm.DB) {
					if !*armed || tx.Statement.Table != "security_configs" {
						return
					}
					queries++
					if queries == 2 {
						tx.Error = injected
					}
				})
			},
			wantMsg: "read security config",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			var before models.SecurityConfig
			if c.seeded {
				before = seedUpdateTestRow(t, db, fullUpdateTestRow())
			}
			armed := false
			require.NoError(t, c.hook(db, &armed))

			armed = true
			got, err := svc.ApplyUpdate(models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_mode":"block"}`))
			armed = false
			require.Error(t, err)
			assert.ErrorIs(t, err, injected)
			assert.Contains(t, err.Error(), c.wantMsg)
			assert.Nil(t, got)

			var stored models.SecurityConfig
			err = db.Where("name = ?", models.DefaultSecurityConfigName).First(&stored).Error
			if !c.seeded {
				assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "a failed create stores nothing")
				return
			}
			require.NoError(t, err)
			requireUpdateTestRowEqual(t, before, stored)
		})
	}
}
