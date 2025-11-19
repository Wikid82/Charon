package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
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

	config, _ := GenerateConfig(hosts)
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
									ReverseProxyHandler("app:8080", false),
								},
							},
							{
								Match: []Match{{Host: []string{"test.com"}}},
								Handle: []Handler{
									ReverseProxyHandler("app2:8080", false),
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
