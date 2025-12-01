package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
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
			UUID:        "test",
			DomainNames: "test.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true)

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

func TestClient_Load_CreateRequestFailure(t *testing.T) {
	// Use baseURL that makes NewRequest return error
	client := NewClient(":bad-url")
	err := client.Load(context.Background(), &Config{})
	require.Error(t, err)
}

func TestClient_Ping_CreateRequestFailure(t *testing.T) {
	client := NewClient(":bad-url")
	err := client.Ping(context.Background())
	require.Error(t, err)
}

func TestClient_GetConfig_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetConfig(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}

func TestClient_GetConfig_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetConfig(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode response")
}

func TestClient_Ping_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "503")
}

func TestClient_RequestCreationErrors(t *testing.T) {
	// Use a control character in URL to force NewRequest error
	client := NewClient("http://example.com" + string(byte(0x7f)))

	err := client.Load(context.Background(), &Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "create request")

	_, err = client.GetConfig(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "create request")

	err = client.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "create request")
}

func TestClient_NetworkErrors(t *testing.T) {
	// Use a closed port to force connection error
	client := NewClient("http://127.0.0.1:0")

	err := client.Load(context.Background(), &Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "execute request")

	_, err = client.GetConfig(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "execute request")
}

func TestClient_Load_MarshalFailure(t *testing.T) {
	// Simulate json.Marshal failure
	orig := jsonMarshalClient
	jsonMarshalClient = func(v interface{}) ([]byte, error) { return nil, fmt.Errorf("marshal error") }
	defer func() { jsonMarshalClient = orig }()

	client := NewClient("http://localhost")
	err := client.Load(context.Background(), &Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "marshal config")
}

type failingTransport struct{}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("round trip failed")
}

func TestClient_Ping_TransportError(t *testing.T) {
	client := NewClient("http://example.com")
	client.httpClient = &http.Client{Transport: &failingTransport{}}
	err := client.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "caddy unreachable")
}
