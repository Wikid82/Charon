package caddy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestGenerateConfig_AccessLogEncoderFiltersQueryToken verifies the access log uses a filter encoder that wraps JSON
// and deletes the token query parameter from request URIs.
func TestGenerateConfig_AccessLogEncoderFiltersQueryToken(t *testing.T) {
	t.Parallel()

	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Contains(t, cfg.Logging.Logs, "access")

	raw, err := json.Marshal(cfg.Logging.Logs["access"].Encoder)
	require.NoError(t, err)

	require.JSONEq(t, `{
		"format": "filter",
		"wrap": {"format": "json"},
		"fields": {
			"request>uri": {
				"filter": "query",
				"actions": [{"type": "delete", "parameter": "token"}]
			}
		}
	}`, string(raw))
}
