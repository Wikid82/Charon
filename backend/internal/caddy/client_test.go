package caddy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
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

	client := NewClientWithExpectedPort(server.URL, expectedPortFromURL(t, server.URL))
	config, _ := GenerateConfig([]models.ProxyHost{
		{
			UUID:        "test",
			DomainNames: "test.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)

	err := client.Load(context.Background(), config)
	require.NoError(t, err)
}

func TestClient_Load_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid config"}`))
	}))
	defer server.Close()

	client := NewClientWithExpectedPort(server.URL, expectedPortFromURL(t, server.URL))
	config := &Config{}

	err := client.Load(context.Background(), config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "400")
}

func TestClient_Ping_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithExpectedPort(server.URL, expectedPortFromURL(t, server.URL))
	err := client.Ping(context.Background())
	require.NoError(t, err)
}

func TestClient_Ping_Unreachable(t *testing.T) {
	client := NewClientWithExpectedPort("http://localhost:9999", 9999)
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

func TestClient_Ping_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClientWithExpectedPort(server.URL, expectedPortFromURL(t, server.URL))
	err := client.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "503")
}

func TestClient_RequestCreationErrors(t *testing.T) {
	// Unsafe base URLs are rejected up-front.
	client := NewClient("http://example.com" + string(byte(0x7f)))

	err := client.Load(context.Background(), &Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "caddy client init failed")
}

func TestClient_NetworkErrors(t *testing.T) {
	// Use a closed port to force connection error
	client := NewClientWithExpectedPort("http://127.0.0.1:1", 1)

	err := client.Load(context.Background(), &Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "execute request")
}

func TestClient_Load_MarshalFailure(t *testing.T) {
	// Simulate json.Marshal failure
	orig := jsonMarshalClient
	jsonMarshalClient = func(v any) ([]byte, error) { return nil, fmt.Errorf("marshal error") }
	defer func() { jsonMarshalClient = orig }()

	client := NewClientWithExpectedPort("http://localhost:2019", 2019)
	err := client.Load(context.Background(), &Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "marshal config")
}

type failingTransport struct{}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("round trip failed")
}

func TestClient_Ping_TransportError(t *testing.T) {
	client := NewClientWithExpectedPort("http://localhost:2019", 2019)
	client.httpClient = &http.Client{Transport: &failingTransport{}}
	err := client.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "caddy unreachable")
}

func TestClient_RequestCreationErrors_FromInvalidResolvedURL(t *testing.T) {
	client := &Client{
		baseURL: &url.URL{Scheme: "http", Host: "example.com\n"},
		initErr: nil,
	}

	err := client.Load(context.Background(), &Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "create request")

	err = client.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "create request")
}

func TestClient_LoadJSON_PostsTheEncodedBody(t *testing.T) {
	const body = `{"apps":{"http":{"servers":{"s":{"listen":[":443"]}}}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/load", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		received, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, body, string(received))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithExpectedPort(server.URL, expectedPortFromURL(t, server.URL))
	require.NoError(t, client.LoadJSON(context.Background(), []byte(body)))
}

func TestClient_LoadJSON_ReportsCaddyRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("unknown field"))
	}))
	defer server.Close()

	client := NewClientWithExpectedPort(server.URL, expectedPortFromURL(t, server.URL))
	err := client.LoadJSON(context.Background(), []byte(`{}`))
	require.ErrorContains(t, err, "caddy returned status 400: unknown field")
}
