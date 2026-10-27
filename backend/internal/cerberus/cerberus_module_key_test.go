package cerberus_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/models"
)

// wafRequestsTotal reads the WAF request counter through a fresh registry, since the counter is not exported.
func wafRequestsTotal(t *testing.T) float64 {
	t.Helper()
	registry := prometheus.NewRegistry()
	metrics.Register(registry)
	families, err := registry.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() == "charon_waf_requests_total" {
			return family.GetMetric()[0].GetCounter().GetValue()
		}
	}
	require.FailNow(t, "charon_waf_requests_total was not gathered")
	return 0
}

// TestCerberusWAFMetric_FollowsResolvedModuleState describes the WAF metric in the API middleware reading the same
// resolved module state as the proxy, not only the static configuration. A settings key wins, then the default row,
// then the static WAF mode.
func TestCerberusWAFMetric_FollowsResolvedModuleState(t *testing.T) {
	cases := []struct {
		name       string
		rowWAF     string
		setting    string
		staticWAF  string
		wantCounts bool
	}{
		{name: "row block counts with static disabled", rowWAF: "block", staticWAF: "disabled", wantCounts: true},
		{name: "false key suppresses a block row", rowWAF: "block", setting: "false", staticWAF: "block", wantCounts: false},
		{name: "true key counts over a disabled row", rowWAF: "disabled", setting: "true", staticWAF: "", wantCounts: true},
		{name: "disabled row overrides static block", rowWAF: "disabled", staticWAF: "block", wantCounts: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupDB(t)
			require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: tc.rowWAF}).Error)
			if tc.setting != "" {
				require.NoError(t, db.Create(&models.Setting{Key: models.SecurityWAFEnabledSettingKey, Value: tc.setting, Category: "security", Type: "bool"}).Error)
			}

			c := cerberus.New(config.SecurityConfig{CerberusEnabled: true, WAFMode: tc.staticWAF}, db)

			before := wafRequestsTotal(t)
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			c.Middleware()(ctx)
			require.False(t, ctx.IsAborted())

			delta := wafRequestsTotal(t) - before
			if tc.wantCounts {
				require.Equal(t, float64(1), delta)
			} else {
				require.Equal(t, float64(0), delta)
			}
		})
	}
}
