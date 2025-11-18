package caddy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func TestClient_Load_Success(t *testing.T) {
	// Mock Caddy admin API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/load", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	config, _ := GenerateConfig([]models.ProxyHost{
		{
			UUID:       "test",
			Domain:     "test.com",
			TargetHost: "app",
			TargetPort: 8080,
		},
	})

	err := client.Load(context.Background(), config)
	require.NoError(t, err)
}

func TestClient_Load_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid config"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	config := &Config{}

	err := client.Load(context.Background(), config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "400")
}

func TestClient_GetConfig_Success(t *testing.T) {
	testConfig := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"test": {Listen: []string{":80"}},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/config/", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(testConfig)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	config, err := client.GetConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotNil(t, config.Apps.HTTP)
}

func TestClient_Ping_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Ping(context.Background())
	require.NoError(t, err)
}

func TestClient_Ping_Unreachable(t *testing.T) {
	client := NewClient("http://localhost:9999")
	err := client.Ping(context.Background())
	require.Error(t, err)
}
