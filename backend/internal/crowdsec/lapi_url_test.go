package crowdsec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLocalLAPIURL(t *testing.T) {
	local := []string{
		LocalLAPIURL,
		"http://127.0.0.1:8085/",
		"http://localhost:8085",
		"HTTP://LOCALHOST:8085",
		"https://localhost:8085",
		"http://[::1]:8085",
	}
	for _, raw := range local {
		assert.True(t, IsLocalLAPIURL(raw), "%q names the local engine", raw)
	}

	notLocal := []string{
		"",
		"not a url",
		"http://[::1",
		"ftp://127.0.0.1:8085",
		"http://127.0.0.1:9",
		"http://127.0.0.1",
		"http://10.0.0.5:8085",
		"http://127.0.0.1:8085/api",
		"http://user@127.0.0.1:8085",
		"http://127.0.0.1:8085?x=1",
	}
	for _, raw := range notLocal {
		assert.False(t, IsLocalLAPIURL(raw), "%q does not name the local engine", raw)
	}
}
