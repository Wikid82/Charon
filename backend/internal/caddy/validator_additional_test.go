package caddy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate_NilConfig(t *testing.T) {
	err := Validate(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config cannot be nil")
}

func TestValidateHandler_MissingHandlerField(t *testing.T) {
	// Handler without a 'handler' key
	h := Handler{"foo": "bar"}
	err := validateHandler(h)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing 'handler' field")
}

func TestValidateHandler_UnknownHandlerAllowed(t *testing.T) {
	// Unknown handler type should be allowed
	h := Handler{"handler": "custom_handler"}
	err := validateHandler(h)
	require.NoError(t, err)
}

func TestValidateHandler_FileServerAndStaticResponseAllowed(t *testing.T) {
	h1 := Handler{"handler": "file_server"}
	err := validateHandler(h1)
	require.NoError(t, err)

	h2 := Handler{"handler": "static_response"}
	err = validateHandler(h2)
	require.NoError(t, err)
}

func TestValidateRoute_InvalidHandler(t *testing.T) {
	config := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"srv": {
						Listen: []string{":80"},
						Routes: []*Route{{
							Match:  []Match{{Host: []string{"test.invalid"}}},
							Handle: []Handler{{"foo": "bar"}},
						}},
					},
				},
			},
		},
	}
	err := Validate(config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid handler")
}

func TestValidateListenAddr_InvalidHostName(t *testing.T) {
	err := validateListenAddr("example.com:80")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid IP address")
}

func TestValidateListenAddr_InvalidPortNonNumeric(t *testing.T) {
	err := validateListenAddr(":abc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid port")
}

func TestValidate_MarshalError(t *testing.T) {
	// stub jsonMarshalValidate to cause Marshal error
	orig := jsonMarshalValidate
	jsonMarshalValidate = func(v interface{}) ([]byte, error) { return nil, fmt.Errorf("marshal error") }
	defer func() { jsonMarshalValidate = orig }()

	cfg := &Config{Apps: Apps{HTTP: &HTTPApp{Servers: map[string]*Server{"srv": {Listen: []string{":80"}, Routes: []*Route{{Match: []Match{{Host: []string{"x.com"}}}, Handle: []Handler{{"handler": "file_server"}}}}}}}}}
	err := Validate(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config cannot be marshalled")
}
