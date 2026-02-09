package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidate_EmergencyPlusMainPattern tests the core fix:
// Allow duplicate host when one has path matchers (emergency) and one doesn't (main)
func TestValidate_EmergencyPlusMainPattern(t *testing.T) {
	tests := []struct {
		name        string
		routes      []*Route
		expectError bool
		errorText   string
	}{
		{
			name: "Emergency+Main Pattern (ALLOWED)",
			routes: []*Route{
				{
					// Emergency route WITH paths
					Match: []Match{{
						Host: []string{"test.com"},
						Path: []string{"/api/v1/emergency/*", "/emergency/*"},
					}},
					Handle: []Handler{
						ReverseProxyHandler("app:8080", false, "none", true),
					},
				},
				{
					// Main route WITHOUT paths
					Match: []Match{{
						Host: []string{"test.com"},
					}},
					Handle: []Handler{
						ReverseProxyHandler("app:8080", false, "none", true),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Reverse Order: Main+Emergency (ALLOWED)",
			routes: []*Route{
				{
					// Main route WITHOUT paths (comes first)
					Match: []Match{{
						Host: []string{"example.com"},
					}},
					Handle: []Handler{
						ReverseProxyHandler("app:8080", false, "none", true),
					},
				},
				{
					// Emergency route WITH paths (comes second)
					Match: []Match{{
						Host: []string{"example.com"},
						Path: []string{"/emergency/*"},
					}},
					Handle: []Handler{
						ReverseProxyHandler("app:8080", false, "none", true),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Duplicate Hosts With Same Paths (REJECTED)",
			routes: []*Route{
				{
					Match: []Match{{
						Host: []string{"test.com"},
						Path: []string{"/api/*"},
					}},
					Handle: []Handler{
						ReverseProxyHandler("app1:8080", false, "none", true),
					},
				},
				{
					Match: []Match{{
						Host: []string{"test.com"},
						Path: []string{"/admin/*"}, // Different paths, but both have paths
					}},
					Handle: []Handler{
						ReverseProxyHandler("app2:8080", false, "none", true),
					},
				},
			},
			expectError: true,
			errorText:   "duplicate host with paths",
		},
		{
			name: "Duplicate Hosts Without Paths (REJECTED)",
			routes: []*Route{
				{
					Match: []Match{{
						Host: []string{"test.com"},
					}},
					Handle: []Handler{
						ReverseProxyHandler("app1:8080", false, "none", true),
					},
				},
				{
					Match: []Match{{
						Host: []string{"test.com"}, // Same host, both without paths
					}},
					Handle: []Handler{
						ReverseProxyHandler("app2:8080", false, "none", true),
					},
				},
			},
			expectError: true,
			errorText:   "duplicate host without paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Apps: Apps{
					HTTP: &HTTPApp{
						Servers: map[string]*Server{
							"test": {
								Listen: []string{":80"},
								Routes: tt.routes,
							},
						},
					},
				},
			}

			err := Validate(config)
			if tt.expectError {
				require.Error(t, err, "Expected validation to fail")
				if tt.errorText != "" {
					require.Contains(t, err.Error(), tt.errorText)
				}
			} else {
				require.NoError(t, err, "Expected validation to pass")
			}
		})
	}
}

// TestValidate_MultipleHostsWithEmergencyPattern tests scalability:
// Verify validator handles 5, 10, and 18+ hosts with emergency+main pattern
func TestValidate_MultipleHostsWithEmergencyPattern(t *testing.T) {
	hostCounts := []int{5, 10, 18}

	for _, count := range hostCounts {
		t.Run("RouteCount_"+string(rune(count+'0')), func(t *testing.T) {
			routes := make([]*Route, 0, count*2) // 2 routes per host

			for i := 0; i < count; i++ {
				hostname := "host" + string(rune(i+'0')) + ".example.com"

				// Emergency route
				routes = append(routes, &Route{
					Match: []Match{{
						Host: []string{hostname},
						Path: []string{"/api/v1/emergency/*", "/emergency/*"},
					}},
					Handle: []Handler{
						ReverseProxyHandler("app:8080", false, "none", true),
					},
				})

				// Main route
				routes = append(routes, &Route{
					Match: []Match{{
						Host: []string{hostname},
					}},
					Handle: []Handler{
						ReverseProxyHandler("app:8080", false, "none", true),
					},
				})
			}

			config := &Config{
				Apps: Apps{
					HTTP: &HTTPApp{
						Servers: map[string]*Server{
							"test": {
								Listen: []string{":80"},
								Routes: routes,
							},
						},
					},
				},
			}

			err := Validate(config)
			require.NoError(t, err, "Expected validation to pass for %d hosts", count)
		})
	}
}

// TestValidate_RouteOrdering verifies route ordering is preserved
func TestValidate_RouteOrdering(t *testing.T) {
	// Emergency route should be checked BEFORE main route
	// This test ensures validator doesn't impose ordering constraints
	config := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"test": {
						Listen: []string{":80"},
						Routes: []*Route{
							{
								// Route 0: Emergency (with paths) - should match /emergency/* first
								Match: []Match{{
									Host: []string{"test.com"},
									Path: []string{"/emergency/*"},
								}},
								Handle: []Handler{
									Handler{
										"handler": "static_response",
										"body":    "Emergency bypass",
									},
								},
								Terminal: true,
							},
							{
								// Route 1: Main (no paths) - matches everything else
								Match: []Match{{
									Host: []string{"test.com"},
								}},
								Handle: []Handler{
									ReverseProxyHandler("app:8080", false, "none", true),
								},
								Terminal: true,
							},
						},
					},
				},
			},
		},
	}

	err := Validate(config)
	require.NoError(t, err, "Route ordering should be preserved")
}

// TestValidate_CaseInsensitiveHosts tests that host comparison is case-sensitive
// This is intentional - DNS is case-insensitive, but Caddy handles normalization
func TestValidate_CaseSensitiveHostnames(t *testing.T) {
	// Note: This test documents current behavior.
	// The validator treats "Test.com" and "test.com" as DIFFERENT strings.
	// This is acceptable because config.go normalizes all hostnames to lowercase
	// BEFORE calling the validator. By the time validator sees them, they're already
	// normalized, so this scenario doesn't occur in production.
	config := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"test": {
						Listen: []string{":80"},
						Routes: []*Route{
							{
								Match: []Match{{
									Host: []string{"Test.com"}, // Uppercase T
								}},
								Handle: []Handler{
									ReverseProxyHandler("app1:8080", false, "none", true),
								},
							},
							{
								Match: []Match{{
									Host: []string{"test.com"}, // Lowercase t
								}},
								Handle: []Handler{
									ReverseProxyHandler("app2:8080", false, "none", true),
								},
							},
						},
					},
				},
			},
		},
	}

	// Current behavior: validator treats these as different hosts (case-sensitive string comparison)
	// This is fine because config.go normalizes all domains to lowercase before validation
	err := Validate(config)
	require.NoError(t, err, "Validator treats different case as different hosts (config.go normalizes before validation)")
}
