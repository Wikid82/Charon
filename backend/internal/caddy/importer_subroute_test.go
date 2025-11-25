package caddy

import (
	"encoding/json"
	"testing"
)

func TestExtractHandlers_Subroute(t *testing.T) {
	// Test JSON that mimics the plex.caddy structure
	rawJSON := `{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [{
							"match": [{"host": ["plex.hatfieldhosted.com"]}],
							"handle": [{
								"handler": "subroute",
								"routes": [{
									"handle": [{
										"handler": "headers"
									}, {
										"handler": "reverse_proxy",
										"upstreams": [{"dial": "100.99.23.57:32400"}]
									}]
								}]
							}]
						}]
					}
				}
			}
		}
	}`

	var config CaddyConfig
	err := json.Unmarshal([]byte(rawJSON), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	importer := NewImporter("caddy")
	route := config.Apps.HTTP.Servers["srv0"].Routes[0]

	handlers := importer.extractHandlers(route.Handle)

	// We should get 2 handlers: headers and reverse_proxy
	if len(handlers) != 2 {
		t.Fatalf("Expected 2 handlers, got %d", len(handlers))
	}

	if handlers[0].Handler != "headers" {
		t.Errorf("Expected first handler to be 'headers', got '%s'", handlers[0].Handler)
	}

	if handlers[1].Handler != "reverse_proxy" {
		t.Errorf("Expected second handler to be 'reverse_proxy', got '%s'", handlers[1].Handler)
	}

	// Check if upstreams are preserved
	if handlers[1].Upstreams == nil {
		t.Fatal("Upstreams should not be nil")
	}

	upstreams, ok := handlers[1].Upstreams.([]interface{})
	if !ok {
		t.Fatal("Upstreams should be []interface{}")
	}

	if len(upstreams) == 0 {
		t.Fatal("Upstreams should not be empty")
	}

	upstream, ok := upstreams[0].(map[string]interface{})
	if !ok {
		t.Fatal("First upstream should be map[string]interface{}")
	}

	dial, ok := upstream["dial"].(string)
	if !ok {
		t.Fatal("Dial should be a string")
	}

	if dial != "100.99.23.57:32400" {
		t.Errorf("Expected dial to be '100.99.23.57:32400', got '%s'", dial)
	}
}
