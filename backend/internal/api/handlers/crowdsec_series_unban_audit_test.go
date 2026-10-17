package handlers

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func decisionRows(t *testing.T, h *CrowdsecHandler) []models.SecurityDecision {
	t.Helper()
	var rows []models.SecurityDecision
	require.NoError(t, h.DB.Order("id").Find(&rows).Error)
	return rows
}

// A successful unban leaves an audit row, like a ban does.
func TestUnbanIP_WritesAnAuditRow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, target, fixture, want string
	}{
		{"address", "192.0.2.11", "decisions_delete", "192.0.2.11"},
		{"range", url.PathEscape("198.51.100.0/24"), "decisions_delete_range_r", "198.51.100.0/24"},
		{"ipv6", url.PathEscape("2001:db8::1"), "decisions_delete_ipv6", "2001:db8::1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := pr4Handler(t, pr4Answer(t, tc.fixture), t.TempDir())

			w := pr4Serve(pr4Router(h), http.MethodDelete, pr4BanURL+"/"+tc.target, "")

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			rows := decisionRows(t, h)
			require.Len(t, rows, 1)
			assert.Equal(t, tc.want, rows[0].IP)
			assert.Equal(t, "unblock", rows[0].Action)
			assert.Equal(t, "crowdsec", rows[0].Source)
			assert.Equal(t, "manual unban", rows[0].RuleID)
		})
	}
}

func TestUnbanIP_WritesNoAuditRowWhenNothingWasRemoved(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		fixture string
		status  int
	}{
		"nothing matched": {"decisions_delete_missing", http.StatusNotFound},
		"cscli failed":    {"decisions_delete_badip", http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := pr4Handler(t, pr4Answer(t, tc.fixture), t.TempDir())

			w := pr4Serve(pr4Router(h), http.MethodDelete, pr4BanURL+"/192.0.2.250", "")

			require.Equal(t, tc.status, w.Code, w.Body.String())
			assert.Empty(t, decisionRows(t, h))
		})
	}
}

// Exact matching: a covering range is not removed by its address and the answer says so.
func TestUnbanIP_NotFoundTextExplainsExactMatching(t *testing.T) {
	t.Parallel()
	h := pr4Handler(t, pr4Answer(t, "decisions_delete_range_absent"), t.TempDir())

	w := pr4Serve(pr4Router(h), http.MethodDelete, pr4BanURL+"/198.51.100.0%2F24", "")

	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	body := pr4Body(t, w)
	assert.Equal(t, "decision_not_found", body["error_code"])
	assert.Equal(t, "no decision with exactly this address or range", body["error"])
}

// A range ban is stored with its CIDR in the address column; the dashboard lists and exports it as written.
func TestSecurityDecisionsShowCIDRValues(t *testing.T) {
	t.Parallel()
	h := pr4Handler(t, pr4Answer(t, "decisions_add_range_r"), t.TempDir())
	r := pr4Router(h)
	require.Equal(t, http.StatusOK, pr4Serve(r, http.MethodPost, pr4BanURL, pr4BanBody(t, "198.51.100.0/24", "4h", "")).Code)
	require.NoError(t, h.DB.Create(&models.SecurityDecision{
		UUID: "11111111-1111-4111-8111-111111111111", IP: "2001:db8:1::/48", Action: "block", Source: "crowdsec",
		Scenario: "manual", Country: "ZZ", CreatedAt: time.Now(),
	}).Error)

	t.Run("top IPs", func(t *testing.T) {
		w := pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?range=24h", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body struct {
			IPs []struct {
				IP string `json:"ip"`
			} `json:"ips"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		got := map[string]bool{}
		for _, ip := range body.IPs {
			got[ip.IP] = true
		}
		assert.Contains(t, got, "198.51.100.0/24")
		assert.Contains(t, got, "2001:db8:1::/48")
	})

	t.Run("csv export", func(t *testing.T) {
		w := pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=csv&range=24h&source=crowdsec", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		rows, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
		require.NoError(t, err)
		ips := map[string]bool{}
		for _, row := range rows[1:] {
			ips[row[1]] = true
		}
		assert.True(t, ips["198.51.100.0/24"], "%v", ips)
		assert.True(t, ips["2001:db8:1::/48"], "%v", ips)
	})

	t.Run("json export", func(t *testing.T) {
		w := pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=json&range=24h&source=crowdsec", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var rows []models.SecurityDecision
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows))
		ips := map[string]bool{}
		for _, row := range rows {
			ips[row.IP] = true
		}
		assert.True(t, ips["198.51.100.0/24"], "%v", ips)
		assert.True(t, ips["2001:db8:1::/48"], "%v", ips)
	})
}

// The unban row is exported with its action.
func TestUnbanRowAppearsInTheExport(t *testing.T) {
	t.Parallel()
	h := pr4Handler(t, pr4Answer(t, "decisions_delete_range_r"), t.TempDir())
	r := pr4Router(h)
	require.Equal(t, http.StatusOK, pr4Serve(r, http.MethodDelete, pr4BanURL+"/198.51.100.0%2F24", "").Code)

	w := pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=json&range=24h&source=crowdsec", "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var rows []models.SecurityDecision
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "unblock", rows[0].Action)
	assert.Equal(t, "198.51.100.0/24", rows[0].IP)
}

// An unban row records a lifted ban, so the attack totals and the top addresses leave it out.
func TestDashboardLeavesOutUnbanRows(t *testing.T) {
	t.Parallel()
	h := pr4Handler(t, pr4Answer(t, "decisions_delete"), t.TempDir())
	r := pr4Router(h)
	require.NoError(t, h.DB.Create(&models.SecurityDecision{
		UUID: "22222222-2222-4222-8222-222222222222", IP: "192.0.2.11", Action: "block", Source: "crowdsec",
		Scenario: "manual", CreatedAt: time.Now(),
	}).Error)
	require.Equal(t, http.StatusOK, pr4Serve(r, http.MethodDelete, pr4BanURL+"/192.0.2.11", "").Code)
	require.Len(t, decisionRows(t, h), 2)

	t.Run("top IPs", func(t *testing.T) {
		w := pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?range=24h", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body struct {
			IPs []struct {
				IP    string `json:"ip"`
				Count int64  `json:"count"`
			} `json:"ips"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Len(t, body.IPs, 1)
		assert.Equal(t, int64(1), body.IPs[0].Count)
	})

	t.Run("summary", func(t *testing.T) {
		w := pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=24h", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body struct {
			Total  int64 `json:"total_decisions"`
			Unique int64 `json:"unique_ips"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, int64(1), body.Total, w.Body.String())
		assert.Equal(t, int64(1), body.Unique)
	})
}
