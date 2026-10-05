package crowdsec

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/network"
)

func TestNewHubHTTPClient_RejectsReservedDestinations(t *testing.T) {
	transport, ok := newHubHTTPClient(time.Second).Transport.(*http.Transport)
	if !ok {
		t.Fatal("unexpected transport type")
	}
	for _, addr := range []string{"100.64.0.1:9", "198.18.0.1:9", "[2002::1]:9", "10.0.0.1:9"} {
		conn, err := transport.DialContext(context.Background(), "tcp", addr)
		if err == nil {
			_ = conn.Close()
			t.Errorf("dial %s succeeded", addr)
			continue
		}
		if !errors.Is(err, network.ErrBlockedAddress) {
			t.Errorf("dial %s: expected policy rejection, got %v", addr, err)
		}
	}
}
