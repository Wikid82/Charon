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

// TestGenerateConfig_DefaultLogSkipsAccessEntries verifies Caddy's default log, which the container captures on
// stdout/stderr, excludes access entries so raw request URIs never reach it. Its level and writer are left at
// Caddy's defaults so runtime logging is unchanged.
func TestGenerateConfig_DefaultLogSkipsAccessEntries(t *testing.T) {
	t.Parallel()

	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Contains(t, cfg.Logging.Logs, "default")

	defaultLog := cfg.Logging.Logs["default"]
	require.Nil(t, defaultLog.Writer, "the default log keeps Caddy's stderr writer")
	require.Nil(t, defaultLog.Encoder, "the default log keeps Caddy's default encoder")

	raw, err := json.Marshal(defaultLog)
	require.NoError(t, err)
	require.JSONEq(t, `{"level": "INFO", "exclude": ["http.log.access.access_log"]}`, string(raw))
}
