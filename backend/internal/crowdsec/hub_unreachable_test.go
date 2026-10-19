package crowdsec

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstallFailureClassifiesNetworkText(t *testing.T) {
	t.Parallel()
	meta := CachedPreset{Type: "collections", Name: "crowdsecurity/nginx"}
	execErr := errors.New("exit status 1")

	cases := map[string]struct {
		out  string
		want error
	}{
		"dial error":            {"Error: Get \"https://cdn-hub.example\": dial tcp: lookup cdn-hub.example: no such host", ErrHubUnreachable},
		"upper case":            {"ERROR: Connection Refused", ErrHubUnreachable},
		"stale item":            {"Error: can't find 'crowdsecurity/nginx' in collections", ErrPresetCacheStale},
		"stale + lookup":        {"invalid hub index: lookup failed", ErrPresetCacheStale},
		"other failure":         {"Error: disk full", nil},
		"resolver text":         {"Error: lookup hub-data.crowdsec.net on 127.0.0.11:53: server misbehaving", ErrHubUnreachable},
		"latest version":        {"Error: unable to retrieve latest crowdsec version", ErrHubUnreachable},
		"item name with lookup": {"Error: cannot install parsers acme/lookup-table: disk full", nil},
		"file name with lookup": {"Error: open /etc/crowdsec/parsers/lookup.yaml: permission denied", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := installFailure(meta, []byte(tc.out), execErr)
			require.Error(t, err)
			if tc.want == nil {
				require.NotErrorIs(t, err, ErrHubUnreachable)
				require.NotErrorIs(t, err, ErrPresetCacheStale)
				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestFetchIndexReportsAnUnreachableHub(t *testing.T) {
	t.Parallel()
	newService := func(cscliOut []byte, cscliErr error) *HubService {
		exec := &recordingExec{outputs: map[string][]byte{"cscli hub list -a -o json": cscliOut}}
		if cscliErr != nil {
			exec.errors = map[string]error{"cscli hub list -a -o json": cscliErr}
		}
		svc := NewHubService(exec, nil, t.TempDir())
		svc.HubBaseURL = "https://hub.crowdsec.net"
		svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp: no such host")
		})}
		return svc
	}

	t.Run("cscli fails too", func(t *testing.T) {
		t.Parallel()
		_, err := newService(nil, errors.New("exit status 1")).FetchIndex(context.Background())
		require.ErrorIs(t, err, ErrHubUnreachable)
	})

	t.Run("cscli prints no json", func(t *testing.T) {
		t.Parallel()
		_, err := newService([]byte("Error: nothing to list"), nil).FetchIndex(context.Background())
		require.ErrorIs(t, err, ErrHubUnreachable)
	})

	t.Run("a cancelled request is not an outage", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := newService(nil, errors.New("exit status 1")).FetchIndex(ctx)
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrHubUnreachable)
	})
}
