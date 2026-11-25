package remotestorage

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebDAVUploader_OtherSchemeRefused(t *testing.T) {
	_, err := newWebDAVUploader(WebDAVConfig{URL: "ftp://203.0.113.10/dav/", AllowInsecureHTTP: true}, WebDAVSecrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http or https")
}

// A saved http:// target without the flag must still build an uploader, so uploads keep working until the
// start-up backfill sets the flag.
func TestNewWebDAVUploader_HTTPWithoutFlagStillBuilds(t *testing.T) {
	uploader, err := newWebDAVUploader(WebDAVConfig{URL: "http://203.0.113.10/dav/"}, WebDAVSecrets{})
	require.NoError(t, err)
	assert.NotNil(t, uploader)
}

func TestWebDAVConfig_AllowInsecureHTTPJSON(t *testing.T) {
	flagged, err := json.Marshal(WebDAVConfig{URL: "http://203.0.113.10/dav/", AllowInsecureHTTP: true})
	require.NoError(t, err)
	assert.Contains(t, string(flagged), `"allow_insecure_http":true`)

	plain, err := json.Marshal(WebDAVConfig{URL: "https://203.0.113.10/dav/"})
	require.NoError(t, err)
	assert.NotContains(t, string(plain), "allow_insecure_http")
}
