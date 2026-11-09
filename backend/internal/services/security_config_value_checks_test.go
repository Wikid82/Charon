package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// valueCheckCase is one save body and the outcome it must have: refused with a message containing wantMsg, or accepted.
type valueCheckCase struct {
	name    string
	body    string
	refused bool
	wantMsg string
}

// runValueCheckCases applies each body to the fully populated default row and checks the outcome. A refused save must
// store nothing: the row is compared with its pre-save state afterwards.
func runValueCheckCases(t *testing.T, cases []valueCheckCase) {
	t.Helper()
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			before := seedUpdateTestRow(t, db, fullUpdateTestRow())

			got, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, c.body))
			if !c.refused {
				require.NoError(t, err)
				assert.NotNil(t, got)
				return
			}
			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), c.wantMsg)
			requireUpdateTestRowEqual(t, before, loadUpdateTestRow(t, db))
		})
	}
}

func TestApplyUpdate_WAFParanoiaLevelRange(t *testing.T) {
	runValueCheckCases(t, []valueCheckCase{
		{name: "zero is refused", body: `{"waf_paranoia_level":0}`, refused: true, wantMsg: "waf_paranoia_level must be between 1 and 4"},
		{name: "five is refused", body: `{"waf_paranoia_level":5}`, refused: true, wantMsg: "waf_paranoia_level must be between 1 and 4"},
		{name: "ninety-nine is refused", body: `{"waf_paranoia_level":99}`, refused: true, wantMsg: "waf_paranoia_level must be between 1 and 4"},
		{name: "negative is refused", body: `{"waf_paranoia_level":-1}`, refused: true, wantMsg: "waf_paranoia_level must be between 1 and 4"},
		{name: "one is accepted", body: `{"waf_paranoia_level":1}`},
		{name: "four is accepted", body: `{"waf_paranoia_level":4}`},
	})
}

func TestApplyUpdate_RateLimitNumberRanges(t *testing.T) {
	runValueCheckCases(t, []valueCheckCase{
		{name: "requests minus one is refused", body: `{"rate_limit_requests":-1}`, refused: true, wantMsg: "rate_limit_requests must be between 0 and 100000"},
		{name: "requests above ceiling is refused", body: `{"rate_limit_requests":100001}`, refused: true, wantMsg: "rate_limit_requests must be between 0 and 100000"},
		{name: "requests zero is unset and accepted", body: `{"rate_limit_requests":0}`},
		{name: "requests at ceiling is accepted", body: `{"rate_limit_requests":100000}`},
		{name: "burst minus one is refused", body: `{"rate_limit_burst":-1}`, refused: true, wantMsg: "rate_limit_burst must be between 0 and 10000"},
		{name: "burst above ceiling is refused", body: `{"rate_limit_burst":10001}`, refused: true, wantMsg: "rate_limit_burst must be between 0 and 10000"},
		{name: "burst zero is unset and accepted", body: `{"rate_limit_burst":0}`},
		{name: "burst at ceiling is accepted", body: `{"rate_limit_burst":10000}`},
		{name: "window minus one is refused", body: `{"rate_limit_window_sec":-1}`, refused: true, wantMsg: "rate_limit_window_sec must be between 0 and 86400"},
		{name: "window above ceiling is refused", body: `{"rate_limit_window_sec":86401}`, refused: true, wantMsg: "rate_limit_window_sec must be between 0 and 86400"},
		{name: "window zero is unset and accepted", body: `{"rate_limit_window_sec":0}`},
		{name: "window at ceiling is accepted", body: `{"rate_limit_window_sec":86400}`},
	})
}

// rate_limit_mode is exact and lowercase: the proxy compares with == and the resolver folds case, so only the exact
// values are stored. The check runs whenever the field is present, including with rate_limit_enable false.
func TestApplyUpdate_RateLimitModeIsExactLowercase(t *testing.T) {
	runValueCheckCases(t, []valueCheckCase{
		{name: "capitalised enabled is refused", body: `{"rate_limit_mode":"Enabled"}`, refused: true, wantMsg: `rate_limit_mode must be "", "disabled" or "enabled"`},
		{name: "unknown word is refused", body: `{"rate_limit_mode":"garbage"}`, refused: true, wantMsg: `rate_limit_mode must be "", "disabled" or "enabled"`},
		{name: "garbage with enable false is refused", body: `{"rate_limit_enable":false,"rate_limit_mode":"garbage"}`, refused: true, wantMsg: `rate_limit_mode must be "", "disabled" or "enabled"`},
		{name: "garbage with enable true is refused", body: `{"rate_limit_enable":true,"rate_limit_mode":"garbage"}`, refused: true, wantMsg: `rate_limit_mode must be "", "disabled" or "enabled"`},
		{name: "enabled is accepted", body: `{"rate_limit_mode":"enabled"}`},
		{name: "disabled is accepted", body: `{"rate_limit_mode":"disabled"}`},
		{name: "empty is accepted", body: `{"rate_limit_mode":""}`},
		{name: "enable false with enabled mode is accepted", body: `{"rate_limit_enable":false,"rate_limit_mode":"enabled"}`},
	})
}

func TestApplyUpdate_RateLimitBypassListEntries(t *testing.T) {
	runValueCheckCases(t, []valueCheckCase{
		{name: "plain IP is accepted", body: `{"rate_limit_bypass_list":"192.0.2.10"}`},
		{name: "CIDR is accepted", body: `{"rate_limit_bypass_list":"192.0.2.0/24"}`},
		{name: "blank entries are skipped", body: `{"rate_limit_bypass_list":"192.0.2.0/24, ,198.51.100.1"}`},
		{name: "empty list clears it", body: `{"rate_limit_bypass_list":""}`},
		{name: "non-address entry is refused", body: `{"rate_limit_bypass_list":"not-an-ip"}`, refused: true, wantMsg: "rate_limit_bypass_list must be a comma-separated list of IP addresses or CIDR ranges"},
		{name: "one bad entry refuses the list", body: `{"rate_limit_bypass_list":"192.0.2.0/24,999.1.1.1"}`, refused: true, wantMsg: "rate_limit_bypass_list must be a comma-separated list"},
	})
}

func TestApplyUpdate_CrowdSecAPIURLShape(t *testing.T) {
	runValueCheckCases(t, []valueCheckCase{
		{name: "empty is accepted", body: `{"crowdsec_api_url":""}`},
		{name: "http is accepted", body: `{"crowdsec_api_url":"http://crowdsec.example:8080"}`},
		{name: "https is accepted", body: `{"crowdsec_api_url":"https://crowdsec.example"}`},
		{name: "ftp is refused", body: `{"crowdsec_api_url":"ftp://x"}`, refused: true, wantMsg: "crowdsec_api_url must be empty or an absolute http or https URL with a host"},
		{name: "no scheme is refused", body: `{"crowdsec_api_url":"crowdsec.example:8080"}`, refused: true, wantMsg: "crowdsec_api_url must be empty or an absolute http or https URL with a host"},
		{name: "no host is refused", body: `{"crowdsec_api_url":"http://"}`, refused: true, wantMsg: "crowdsec_api_url must be empty or an absolute http or https URL with a host"},
	})
}

// A refused change stores nothing, including a value that is valid on its own when another field in the same save is not.
func TestApplyUpdate_RefusedValueStoresNothingInTheSameSave(t *testing.T) {
	runValueCheckCases(t, []valueCheckCase{
		{name: "valid mode with bad paranoia", body: `{"waf_mode":"block","waf_paranoia_level":9}`, refused: true, wantMsg: "waf_paranoia_level"},
		{name: "valid enable with bad burst", body: `{"enabled":false,"rate_limit_burst":-5}`, refused: true, wantMsg: "rate_limit_burst"},
	})
}

// The echo rule: a value equal to the stored value is not checked, so an unrelated save never fails on a value that
// was already stored, and a save that posts back the row it read succeeds.
func TestApplyUpdate_EchoOfStoredOutOfRangeValueIsAccepted(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seeded := fullUpdateTestRow()
	seeded.WAFParanoiaLevel = 9
	seeded.RateLimitRequests = 999999999
	seeded.RateLimitMode = "Enabled"
	seeded.RateLimitEnable = false
	seeded.CrowdSecAPIURL = "ftp://old.example"
	seeded.RateLimitBypassList = "not-an-ip"
	seedUpdateTestRow(t, db, seeded)

	t.Run("unrelated save keeps working", func(t *testing.T) {
		got, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_mode":"block"}`))
		require.NoError(t, err)
		assert.Equal(t, "block", got.WAFMode)
	})

	t.Run("echo of every stored out-of-range value is accepted", func(t *testing.T) {
		body := `{"waf_paranoia_level":9,"rate_limit_requests":999999999,"rate_limit_mode":"Enabled",` +
			`"rate_limit_enable":false,"crowdsec_api_url":"ftp://old.example","rate_limit_bypass_list":"not-an-ip"}`
		_, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, body))
		require.NoError(t, err)
	})

	t.Run("changing a stored out-of-range value is refused", func(t *testing.T) {
		_, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_paranoia_level":10}`))
		require.Error(t, err)
	})
}

// A missing row is compared with the model defaults (paranoia 1, mode disabled, everything else zero): a first save of
// paranoia 0 is refused and creates nothing, while zero rate-limit numbers equal the defaults and are accepted.
func TestApplyUpdate_MissingRowValueChecks(t *testing.T) {
	t.Run("paranoia zero on a missing row is refused and nothing is created", func(t *testing.T) {
		db := setupSecurityTestDB(t)
		svc := newTestSecurityService(t, db)

		got, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_paranoia_level":0}`))
		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "waf_paranoia_level must be between 1 and 4")

		var count int64
		require.NoError(t, db.Model(&models.SecurityConfig{}).Count(&count).Error)
		assert.Equal(t, int64(0), count, "a refused first save stores no row")
	})

	t.Run("zero rate-limit numbers on a missing row are accepted", func(t *testing.T) {
		db := setupSecurityTestDB(t)
		svc := newTestSecurityService(t, db)

		got, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName,
			decodeUpdateTestBody(t, `{"rate_limit_requests":0,"rate_limit_burst":0,"rate_limit_window_sec":0}`))
		require.NoError(t, err)
		assert.Equal(t, 0, got.RateLimitRequests)
	})

	t.Run("out-of-range rate-limit number on a missing row is refused", func(t *testing.T) {
		db := setupSecurityTestDB(t)
		svc := newTestSecurityService(t, db)

		_, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"rate_limit_window_sec":86401}`))
		require.Error(t, err)
		var count int64
		require.NoError(t, db.Model(&models.SecurityConfig{}).Count(&count).Error)
		assert.Equal(t, int64(0), count)
	})
}
