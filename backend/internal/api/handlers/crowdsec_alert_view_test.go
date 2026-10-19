package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCSCLIAlerts(t *testing.T) {
	t.Parallel()

	t.Run("source value, first decision and dropped fields", func(t *testing.T) {
		t.Parallel()
		raw := `[{"id":1,"scenario":"s","message":"m","events_count":2,"start_at":"a","stop_at":"b","created_at":"c","machine_id":"x",
			"source":{"ip":"192.0.2.1","value":"192.0.2.0/24"},
			"decisions":[{"duration":"1h","type":"ban","origin":"cscli"},{"duration":"2h","type":"captcha","origin":"crowdsec"}]}]`
		got, err := parseCSCLIAlerts([]byte(raw), 10)
		require.NoError(t, err)
		assert.Equal(t, []alertView{{ID: 1, Scenario: "s", IP: "192.0.2.0/24", Message: "m", EventsCount: 2,
			StartAt: "a", StopAt: "b", CreatedAt: "c", Duration: "1h", Type: "ban", Origin: "cscli"}}, got)
	})

	t.Run("the source ip is used when there is no value", func(t *testing.T) {
		t.Parallel()
		got, err := parseCSCLIAlerts([]byte(`[{"id":2,"source":{"ip":"2001:db8::1"},"decisions":[]}]`), 10)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "2001:db8::1", got[0].IP)
		assert.Empty(t, got[0].Type)
	})

	t.Run("the list is bounded by the limit", func(t *testing.T) {
		t.Parallel()
		got, err := parseCSCLIAlerts([]byte(`[{"id":1},{"id":2},{"id":3}]`), 2)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(2), got[1].ID)
	})

	t.Run("an empty array is an empty list", func(t *testing.T) {
		t.Parallel()
		got, err := parseCSCLIAlerts([]byte("[]"), 10)
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("output that is not JSON is an error", func(t *testing.T) {
		t.Parallel()
		_, err := parseCSCLIAlerts([]byte("no json"), 10)
		require.Error(t, err)
	})
}
