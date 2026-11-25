package caddy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestValidate_EmptyConfig(t *testing.T) {
	config := &Config{}
	err := Validate(config)
	require.NoError(t, err)
}

func TestValidate_ValidConfig(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "test",
			DomainNames: "test.example.com",
			ForwardHost: "10.0.1.100",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	config, _ := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	err := Validate(config)
	require.NoError(t, err)
}

func TestValidate_DuplicateHosts(t *testing.T) {
	config := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"srv": {
						Listen: []string{":80"},
						Routes: []*Route{
							{
								Match: []Match{{Host: []string{"test.com"}}},
								Handle: []Handler{
									ReverseProxyHandler("app:8080", "none", true),
								},
							},
							{
								Match: []Match{{Host: []string{"test.com"}}},
								Handle: []Handler{
									ReverseProxyHandler("app2:8080", "none", true),
								},
							},
						},
					},
				},
			},
		},
	}

	err := Validate(config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate host")
}

func TestValidate_NoListenAddresses(t *testing.T) {
	config := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"srv": {
						Listen: []string{},
						Routes: []*Route{},
					},
				},
			},
		},
	}

	err := Validate(config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no listen addresses")
}

func TestValidate_InvalidPort(t *testing.T) {
	config := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"srv": {
						Listen: []string{":99999"},
						Routes: []*Route{},
					},
				},
			},
		},
	}

	err := Validate(config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

func TestValidate_NoHandlers(t *testing.T) {
	config := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"srv": {
						Listen: []string{":80"},
						Routes: []*Route{
							{
								Match:  []Match{{Host: []string{"test.com"}}},
								Handle: []Handler{},
							},
						},
					},
				},
			},
		},
	}

	err := Validate(config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no handlers")
}

func TestValidateListenAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"Valid", ":80", false},
		{"ValidIP", "127.0.0.1:80", false},
		{"ValidTCP", "tcp/127.0.0.1:80", false},
		{"ValidUDP", "udp/127.0.0.1:80", false},
		{"InvalidFormat", "invalid", true},
		{"InvalidPort", ":99999", true},
		{"InvalidPortNegative", ":-1", true},
		{"InvalidIP", "999.999.999.999:80", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListenAddr(tt.addr)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateReverseProxy(t *testing.T) {
	tests := []struct {
		name    string
		handler Handler
		wantErr bool
	}{
		{
			name: "Valid",
			handler: Handler{
				"handler": "reverse_proxy",
				"upstreams": []map[string]any{
					{"dial": "localhost:8080"},
				},
			},
			wantErr: false,
		},
		{
			name: "MissingUpstreams",
			handler: Handler{
				"handler": "reverse_proxy",
			},
			wantErr: true,
		},
		{
			name: "EmptyUpstreams",
			handler: Handler{
				"handler":   "reverse_proxy",
				"upstreams": []map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "MissingDial",
			handler: Handler{
				"handler": "reverse_proxy",
				"upstreams": []map[string]any{
					{"foo": "bar"},
				},
			},
			wantErr: true,
		},
		{
			name: "InvalidDial",
			handler: Handler{
				"handler": "reverse_proxy",
				"upstreams": []map[string]any{
					{"dial": "invalid"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReverseProxy(tt.handler)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateReverseProxy_DecodedUpstreams(t *testing.T) {
	tests := []struct {
		name      string
		upstreams []any
		wantErr   bool
	}{
		{
			name:      "DecodedObjectsAccepted",
			upstreams: []any{map[string]any{"dial": "localhost:8080"}},
		},
		{
			name:      "DecodedEmptyRefused",
			upstreams: []any{},
			wantErr:   true,
		},
		{
			name:      "DecodedNonObjectRefused",
			upstreams: []any{"localhost:8080"},
			wantErr:   true,
		},
		{
			name:      "DecodedMissingDialRefused",
			upstreams: []any{map[string]any{"foo": "bar"}},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReverseProxy(Handler{"handler": "reverse_proxy", "upstreams": tt.upstreams})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidate_DecodedTopLevelReverseProxy(t *testing.T) {
	// A host whose advanced configuration is a top-level reverse_proxy is decoded
	// from JSON, so its upstreams arrive as []any rather than []map[string]any.
	raw := `{"apps":{"http":{"servers":{"srv0":{"listen":[":443"],"routes":[` +
		`{"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"app:8080"}]}]}` +
		`]}}}}}`

	var cfg Config
	require.NoError(t, json.Unmarshal([]byte(raw), &cfg))
	require.NoError(t, Validate(&cfg))

	bad := `{"apps":{"http":{"servers":{"srv0":{"listen":[":443"],"routes":[` +
		`{"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"no-port"}]}]}` +
		`]}}}}}`
	var badCfg Config
	require.NoError(t, json.Unmarshal([]byte(bad), &badCfg))
	require.Error(t, Validate(&badCfg))
}
