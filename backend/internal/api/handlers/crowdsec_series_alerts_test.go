package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

const alertsURL = "/api/v1/admin/crowdsec/alerts"

// alertsLikeCscli answers `alerts list` like the recorded cscli: --since keeps the alerts that started inside the
// window (one alert for 1h..24h, both for 7d and more) and -l truncates the newest-first list.
func alertsLikeCscli(t *testing.T) *pr4Exec {
	t.Helper()
	return &pr4Exec{respond: func(_ context.Context, args []string) ([]byte, error) {
		args = pr4StripConfig(args)
		fixture := "alerts_list_all_json"
		switch since, _ := pr4Flag(args, "--since"); since {
		case "1h", "6h", "24h":
			fixture = "alerts_list_since_24h_json"
		}
		var alerts []json.RawMessage
		if err := json.Unmarshal(cscliFixture(t, fixture), &alerts); err != nil {
			return nil, err
		}
		if limit, ok := pr4Flag(args, "-l"); ok {
			if n, err := strconv.Atoi(limit); err == nil && n > 0 && n < len(alerts) {
				alerts = alerts[:n]
			}
		}
		return json.Marshal(alerts)
	}}
}

type alertsBody struct {
	Alerts []struct {
		Scenario string `json:"scenario"`
	} `json:"alerts"`
	Total int    `json:"total"`
	Error string `json:"error"`
	Code  string `json:"error_code"`
}

func getAlerts(t *testing.T, ex *pr4Exec, query string) alertsBody {
	t.Helper()
	w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodGet, alertsURL+query, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body alertsBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body
}

func (b alertsBody) scenarios() []string {
	out := make([]string, 0, len(b.Alerts))
	for _, a := range b.Alerts {
		out = append(out, a.Scenario)
	}
	return out
}

// The range of the request is the time window cscli is asked for.
func TestListAlerts_RangeIsPassedAsSince(t *testing.T) {
	t.Parallel()
	for query, want := range map[string]string{
		"":           "24h",
		"?range=1h":  "1h",
		"?range=6h":  "6h",
		"?range=24h": "24h",
		"?range=7d":  "7d",
		"?range=30d": "30d",
	} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			ex := alertsLikeCscli(t)

			getAlerts(t, ex, query)

			since, ok := pr4Flag(pr4StripConfig(ex.lastCall(t)), "--since")
			require.True(t, ok, "no --since flag in %v", ex.lastCall(t))
			assert.Equal(t, want, since)
		})
	}
}

func TestListAlerts_RangeSelectsTheAlertsOfTheWindow(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"manual/new-alert"}, getAlerts(t, alertsLikeCscli(t), "?range=24h").scenarios())
	assert.Equal(t, []string{"manual/new-alert", "manual/old-alert"}, getAlerts(t, alertsLikeCscli(t), "?range=7d").scenarios())
}

// cscli has no offset flag: the server fetches limit+offset alerts (at most 1000) and slices.
func TestListAlerts_OffsetSlicesTheFetchedList(t *testing.T) {
	t.Parallel()
	t.Run("second page", func(t *testing.T) {
		t.Parallel()
		ex := alertsLikeCscli(t)

		body := getAlerts(t, ex, "?range=7d&limit=1&offset=1")

		args := pr4StripConfig(ex.lastCall(t))
		limit, _ := pr4Flag(args, "-l")
		assert.Equal(t, "2", limit, "limit + offset alerts are fetched")
		assert.Equal(t, []string{"manual/old-alert"}, body.scenarios())
		assert.Equal(t, 2, body.Total, "total is the number of fetched alerts")
	})
	t.Run("first page is unchanged", func(t *testing.T) {
		t.Parallel()
		body := getAlerts(t, alertsLikeCscli(t), "?range=7d&limit=1&offset=0")

		assert.Equal(t, []string{"manual/new-alert"}, body.scenarios())
	})
	t.Run("offset past the end", func(t *testing.T) {
		t.Parallel()
		body := getAlerts(t, alertsLikeCscli(t), "?range=7d&limit=10&offset=5")

		assert.Empty(t, body.Alerts)
		assert.Empty(t, body.Error)
	})
	t.Run("the fetch is capped at 1000", func(t *testing.T) {
		t.Parallel()
		ex := alertsLikeCscli(t)

		getAlerts(t, ex, "?limit=200&offset=900")

		limit, _ := pr4Flag(pr4StripConfig(ex.lastCall(t)), "-l")
		assert.Equal(t, "1000", limit)
	})
	t.Run("cscli is never given an offset flag", func(t *testing.T) {
		t.Parallel()
		ex := alertsLikeCscli(t)

		getAlerts(t, ex, "?limit=10&offset=20")

		for _, arg := range ex.lastCall(t) {
			assert.NotContains(t, arg, "offset")
		}
	})
}

// A failed listing is still a 200 with an empty list and the English text, now with a stable code.
func TestSoftListErrorsCarryAnErrorCode(t *testing.T) {
	skipUntilLaterCommit(t)
	t.Parallel()
	over := append([]byte("[]\n"), make([]byte, pr4OverCap)...)
	for i := 3; i < len(over); i++ {
		over[i] = ' '
	}
	failures := map[string]struct {
		code string
		exec func() *pr4Exec
		ctx  time.Duration
	}{
		"timeout": {"cscli_timeout", func() *pr4Exec { return pr4Blocking(true) }, 150 * time.Millisecond},
		"executor output cap": {"cscli_output_too_large", func() *pr4Exec {
			return &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return []byte("[]\n"), crowdsec.ErrOutputLimit }}
		}, 0},
		"parser output cap": {"cscli_output_too_large", func() *pr4Exec {
			return &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return over, nil }}
		}, 0},
		"cscli missing": {"cscli_unavailable", func() *pr4Exec {
			return &pr4Exec{respond: func(context.Context, []string) ([]byte, error) {
				return nil, &exec.Error{Name: "cscli", Err: exec.ErrNotFound}
			}}
		}, 0},
		"cscli fails": {"cscli_unavailable", func() *pr4Exec {
			return &pr4Exec{respond: func(context.Context, []string) ([]byte, error) {
				return []byte("Error: boom"), errors.New("exit status 1")
			}}
		}, 0},
	}
	endpoints := map[string]string{"decisions": "/api/v1/admin/crowdsec/decisions", "alerts": alertsURL}
	for endpoint, target := range endpoints {
		for name, tc := range failures {
			t.Run(endpoint+"/"+name, func(t *testing.T) {
				t.Parallel()
				ctx := context.Background()
				if tc.ctx > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, tc.ctx)
					defer cancel()
				}
				w := pr4ServeCtx(ctx, pr4Router(pr4Handler(t, tc.exec(), t.TempDir())), http.MethodGet, target, "")

				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				body := pr4Body(t, w)
				assert.Equal(t, tc.code, body["error_code"])
				assert.NotEmpty(t, body["error"], "the English text stays as the fallback")
				assert.True(t, strings.Contains(w.Body.String(), `"`+endpoint+`":[]`), "the list is empty: %s", w.Body.String())
			})
		}
	}
}
