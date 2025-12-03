package caddy

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAdvancedConfig_MapWithNestedHandles(t *testing.T) {
	// Build a map with nested 'handle' array containing headers with string values
	raw := map[string]interface{}{
		"handler": "subroute",
		"routes": []interface{}{
			map[string]interface{}{
				"handle": []interface{}{
					map[string]interface{}{
						"handler": "headers",
						"request": map[string]interface{}{
							"set": map[string]interface{}{"Upgrade": "websocket"},
						},
						"response": map[string]interface{}{
							"set": map[string]interface{}{"X-Obj": "1"},
						},
					},
				},
			},
		},
	}

	out := NormalizeAdvancedConfig(raw)
	// Verify nested header values normalized
	outMap, ok := out.(map[string]interface{})
	require.True(t, ok)
	routes := outMap["routes"].([]interface{})
	require.Len(t, routes, 1)
	r := routes[0].(map[string]interface{})
	handles := r["handle"].([]interface{})
	require.Len(t, handles, 1)
	hdr := handles[0].(map[string]interface{})

	// request.set.Upgrade
	req := hdr["request"].(map[string]interface{})
	set := req["set"].(map[string]interface{})
	// Could be []interface{} or []string depending on code path; normalize to []string representation
	switch v := set["Upgrade"].(type) {
	case []interface{}:
		var outArr []string
		for _, it := range v {
			outArr = append(outArr, fmt.Sprintf("%v", it))
		}
		require.Equal(t, []string{"websocket"}, outArr)
	case []string:
		require.Equal(t, []string{"websocket"}, v)
	default:
		t.Fatalf("unexpected type for Upgrade: %T", v)
	}

	// response.set.X-Obj
	resp := hdr["response"].(map[string]interface{})
	rset := resp["set"].(map[string]interface{})
	switch v := rset["X-Obj"].(type) {
	case []interface{}:
		var outArr []string
		for _, it := range v {
			outArr = append(outArr, fmt.Sprintf("%v", it))
		}
		require.Equal(t, []string{"1"}, outArr)
	case []string:
		require.Equal(t, []string{"1"}, v)
	default:
		t.Fatalf("unexpected type for X-Obj: %T", v)
	}
}

func TestNormalizeAdvancedConfig_ArrayTopLevel(t *testing.T) {
	// Top-level array containing a headers handler with array value as []interface{}
	raw := []interface{}{
		map[string]interface{}{
			"handler": "headers",
			"response": map[string]interface{}{
				"set": map[string]interface{}{"X-Obj": []interface{}{"1"}},
			},
		},
	}
	out := NormalizeAdvancedConfig(raw)
	outArr := out.([]interface{})
	require.Len(t, outArr, 1)
	hdr := outArr[0].(map[string]interface{})
	resp := hdr["response"].(map[string]interface{})
	set := resp["set"].(map[string]interface{})
	switch v := set["X-Obj"].(type) {
	case []interface{}:
		var outArr2 []string
		for _, it := range v {
			outArr2 = append(outArr2, fmt.Sprintf("%v", it))
		}
		require.Equal(t, []string{"1"}, outArr2)
	case []string:
		require.Equal(t, []string{"1"}, v)
	default:
		t.Fatalf("unexpected type for X-Obj: %T", v)
	}
}

func TestNormalizeAdvancedConfig_DefaultPrimitives(t *testing.T) {
	// Ensure primitive values remain unchanged
	v := NormalizeAdvancedConfig(42)
	require.Equal(t, 42, v)
	v2 := NormalizeAdvancedConfig("hello")
	require.Equal(t, "hello", v2)
}

func TestNormalizeAdvancedConfig_CoerceNonStandardTypes(t *testing.T) {
	// Use a header value that is numeric and ensure it's coerced to string
	raw := map[string]interface{}{"handler": "headers", "response": map[string]interface{}{"set": map[string]interface{}{"X-Num": 1}}}
	out := NormalizeAdvancedConfig(raw).(map[string]interface{})
	resp := out["response"].(map[string]interface{})
	set := resp["set"].(map[string]interface{})
	// Should be a []string with "1"
	switch v := set["X-Num"].(type) {
	case []interface{}:
		var outArr []string
		for _, it := range v {
			outArr = append(outArr, fmt.Sprintf("%v", it))
		}
		require.Equal(t, []string{"1"}, outArr)
	case []string:
		require.Equal(t, []string{"1"}, v)
	default:
		t.Fatalf("unexpected type for X-Num: %T", v)
	}
}

func TestNormalizeAdvancedConfig_JSONRoundtrip(t *testing.T) {
	// Ensure normalized config can be marshaled back to JSON and unmarshaled
	raw := map[string]interface{}{"handler": "headers", "request": map[string]interface{}{"set": map[string]interface{}{"Upgrade": "websocket"}}}
	out := NormalizeAdvancedConfig(raw)
	b, err := json.Marshal(out)
	require.NoError(t, err)
	// Marshal back and read result
	var parsed interface{}
	require.NoError(t, json.Unmarshal(b, &parsed))
}

func TestNormalizeAdvancedConfig_TopLevelHeaders(t *testing.T) {
	// Top-level 'headers' key should be normalized similar to request/response
	raw := map[string]interface{}{
		"handler": "headers",
		"headers": map[string]interface{}{
			"set": map[string]interface{}{"Upgrade": "websocket"},
		},
	}
	out := NormalizeAdvancedConfig(raw).(map[string]interface{})
	hdrs := out["headers"].(map[string]interface{})
	set := hdrs["set"].(map[string]interface{})
	switch v := set["Upgrade"].(type) {
	case []interface{}:
		var outArr []string
		for _, it := range v {
			outArr = append(outArr, fmt.Sprintf("%v", it))
		}
		require.Equal(t, []string{"websocket"}, outArr)
	case []string:
		require.Equal(t, []string{"websocket"}, v)
	default:
		t.Fatalf("unexpected type for Upgrade: %T", v)
	}
}

func TestNormalizeAdvancedConfig_HeadersAlreadyArray(t *testing.T) {
	// If the header value is already a []string it should be left as-is
	raw := map[string]interface{}{
		"handler": "headers",
		"headers": map[string]interface{}{
			"set": map[string]interface{}{"X-Test": []string{"a", "b"}},
		},
	}
	out := NormalizeAdvancedConfig(raw).(map[string]interface{})
	hdrs := out["headers"].(map[string]interface{})
	set := hdrs["set"].(map[string]interface{})
	switch v := set["X-Test"].(type) {
	case []interface{}:
		var outArr []string
		for _, it := range v {
			outArr = append(outArr, fmt.Sprintf("%v", it))
		}
		require.Equal(t, []string{"a", "b"}, outArr)
	case []string:
		require.Equal(t, []string{"a", "b"}, v)
	default:
		t.Fatalf("unexpected type for X-Test: %T", v)
	}
}

func TestNormalizeAdvancedConfig_MapWithTopLevelHandle(t *testing.T) {
	raw := map[string]interface{}{
		"handler": "subroute",
		"handle": []interface{}{
			map[string]interface{}{
				"handler": "headers",
				"request": map[string]interface{}{"set": map[string]interface{}{"Upgrade": "websocket"}},
			},
		},
	}
	out := NormalizeAdvancedConfig(raw).(map[string]interface{})
	handles := out["handle"].([]interface{})
	require.Len(t, handles, 1)
	hdr := handles[0].(map[string]interface{})
	req := hdr["request"].(map[string]interface{})
	set := req["set"].(map[string]interface{})
	switch v := set["Upgrade"].(type) {
	case []interface{}:
		var outArr []string
		for _, it := range v {
			outArr = append(outArr, fmt.Sprintf("%v", it))
		}
		require.Equal(t, []string{"websocket"}, outArr)
	case []string:
		require.Equal(t, []string{"websocket"}, v)
	default:
		t.Fatalf("unexpected type for Upgrade: %T", v)
	}
}
