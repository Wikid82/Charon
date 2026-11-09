package crowdsec

import (
	"net/url"
	"strings"
)

// LocalLAPIURL is the address of the CrowdSec engine Charon runs on this host. The bouncer key checks always use it.
const LocalLAPIURL = "http://127.0.0.1:8085"

// localLAPIPort is the port the engine's local API listens on.
const localLAPIPort = "8085"

// IsLocalLAPIURL reports whether raw names the local engine: an http or https address of localhost, 127.0.0.1 or
// ::1 on the engine port, with no user, query or path other than a trailing slash.
func IsLocalLAPIURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
	default:
		return false
	}
	return u.Port() == localLAPIPort
}
