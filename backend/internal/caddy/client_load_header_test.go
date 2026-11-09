package caddy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestClient_Load_SendsMustRevalidate(t *testing.T) {
	// Caddy skips /load for an identical config, which would keep data loaded at startup (such as a GeoIP database
	// replaced on disk). The must-revalidate header makes every load reach the running config.
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Cache-Control")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithExpectedPort(server.URL, expectedPortFromURL(t, server.URL))
	config, err := GenerateConfig([]models.ProxyHost{}, t.TempDir(), "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	require.NoError(t, client.Load(context.Background(), config))
	require.Equal(t, "must-revalidate", got)
}
