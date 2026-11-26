package caddy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// Every credential in this file is an obviously fake fixture.

// adminAnswer builds the answer of a fake admin endpoint from the body it received.
type adminAnswer func(received []byte) (status int, text string)

// newFakeAdmin serves /load with the answer built by answer.
func newFakeAdmin(t *testing.T, answer adminAnswer) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ := io.ReadAll(r.Body)
		status, text := answer(received)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(text))
	}))
	t.Cleanup(server.Close)
	return newTestClient(t, server.URL)
}

// loadBody is a config whose DNS provider carries the given token.
func loadBody(t *testing.T, provider string) []byte {
	t.Helper()
	return []byte(`{"apps":{"tls":{"automation":{"policies":[{"issuers":[{"challenges":{"dns":{"provider":` + provider + `}}}]}]}}}}`)
}

// requireAdminError asserts err is an AdminError with the status and returns its message.
func requireAdminError(t *testing.T, err error, status int) string {
	t.Helper()
	require.Error(t, err)
	var adminErr *AdminError
	require.True(t, errors.As(err, &adminErr), "error is %T", err)
	require.Equal(t, status, adminErr.Status)
	return adminErr.Message
}

func TestLoadJSONRedactsTheTokenInTheAdminAnswer(t *testing.T) {
	t.Parallel()
	const token = "FAKE-token-0001"
	body := loadBody(t, `{"name":"cloudflare","api_token":"`+token+`"}`)
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, `{"error":"loading new config: loading tls app module: API token '` + token + `' appears invalid; ensure it's correctly entered and not wrapped in braces nor quotes (dns.providers.cloudflare)"}`
	})

	err := client.LoadJSON(context.Background(), body)

	msg := requireAdminError(t, err, http.StatusBadRequest)
	require.NotContains(t, msg, token)
	require.Contains(t, msg, "appears invalid")
	require.Contains(t, msg, "dns.providers.cloudflare")
	require.Contains(t, msg, "[redacted]")
	require.NotContains(t, err.Error(), token)
	require.Contains(t, err.Error(), "caddy returned status 400: ")
}

func TestLoadJSONRedactsJSONEscapedAndGoQuotedValues(t *testing.T) {
	t.Parallel()
	const token = `FAKE"tok\en-0002`
	body := loadBody(t, `{"name":"cloudflare","api_token":`+strconv.Quote(token)+`}`)
	jsonForm := `FAKE\"tok\\en-0002`
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, `{"error":"raw ` + token + ` json \"` + jsonForm + `\" quoted \"` + jsonForm + `\""}`
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), body), http.StatusBadRequest)
	require.NotContains(t, msg, "tok")
	require.Equal(t, 3, strings.Count(msg, "[redacted]"))
}

func TestLoadJSONRedactsAPluginProviderValueWithAnUnknownField(t *testing.T) {
	t.Parallel()
	const value = "FAKE-plug-0003"
	body := loadBody(t, `{"name":"plugin-dns","zz_field":"`+value+`"}`)
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, `provider rejected "` + value + `"`
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), body), http.StatusBadRequest)
	require.NotContains(t, msg, value)
	require.Contains(t, msg, "provider rejected")
}

// TestLoadJSONRedactsEveryKindOfSentCredential covers the credentials Charon's generated config can carry.
func TestLoadJSONRedactsEveryKindOfSentCredential(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		body  string
		value string
	}{
		{"crowdsec key", `{"crowdsec":{"api_key":"FAKE-crowd-0004"}}`, "FAKE-crowd-0004"},
		{"self hop header", `{"headers":{"request":{"set":{"X-Charon-Self-Hop":["FAKE-hop-0005"]}}}}`, "FAKE-hop-0005"},
		{"advanced header", `{"handler":"headers","request":{"set":{"Authorization":["Bearer FAKE-bearer-0006"]}}}`, "Bearer FAKE-bearer-0006"},
		{"basic account", `{"http_basic":{"accounts":[{"username":"u","password":"FAKE-basic-0007"}]}}`, "FAKE-basic-0007"},
		{"url password", `{"upstream":"https://u:FAKE-url-0008@h.invalid/x"}`, "FAKE-url-0008"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := newFakeAdmin(t, func(received []byte) (int, string) {
				require.Contains(t, string(received), tc.value)
				return http.StatusBadRequest, "rejected: " + tc.value
			})

			msg := requireAdminError(t, client.LoadJSON(context.Background(), []byte(tc.body)), http.StatusBadRequest)
			require.NotContains(t, msg, strings.TrimPrefix(tc.value, "Bearer "))
			require.Contains(t, msg, "rejected")
		})
	}
}

func TestLoadJSONKeepsShortValuesAndBenignKeys(t *testing.T) {
	t.Parallel()
	body := []byte(`{"api_key":"on","token_path":"/etc/charon/token.txt"}`)
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, `api_key on, token_path /etc/charon/token.txt`
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), body), http.StatusBadRequest)
	require.Equal(t, "api_key on, token_path /etc/charon/token.txt", msg)
}

func TestLoadJSONBoundsAHugeAnswer(t *testing.T) {
	t.Parallel()
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, strings.Repeat("x", 1<<20)
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), []byte(`{}`)), http.StatusBadRequest)
	require.LessOrEqual(t, len(msg), maxAdminErrorText+len(" [truncated]"))
	require.True(t, strings.HasSuffix(msg, " [truncated]"))
}

func TestLoadJSONUsesRawTextForANonJSONAnswer(t *testing.T) {
	t.Parallel()
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusInternalServerError, "plain answer from the admin"
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), []byte(`{}`)), http.StatusInternalServerError)
	require.Equal(t, "plain answer from the admin", msg)
}

func TestLoadJSONUsesRawTextForAJSONAnswerWithoutAnErrorField(t *testing.T) {
	t.Parallel()
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, `{"message":"no error field"}`
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), []byte(`{}`)), http.StatusBadRequest)
	require.Equal(t, `{"message":"no error field"}`, msg)
}

func TestLoadJSONKeepsMultiByteTextValidAtTheCap(t *testing.T) {
	t.Parallel()
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, strings.Repeat("é", 3000)
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), []byte(`{}`)), http.StatusBadRequest)
	require.True(t, utf8.ValidString(msg))
}

func TestLoadJSONRedactsPatternsInAnswerOfUnknownValues(t *testing.T) {
	t.Parallel()
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, `{"error":"api_key=FAKE-pat-0009 and \"password\":\"FAKE-pat-0010\" and Bearer FAKE-pat-0011"}`
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), []byte(`{}`)), http.StatusBadRequest)
	require.NotContains(t, msg, "FAKE-pat-00")
	require.Equal(t, 3, strings.Count(msg, "[redacted]"))
}

func TestLoadJSONRedactsAHugeConfigAnswerWithThePatternPass(t *testing.T) {
	t.Parallel()
	// Past the node cap the walk stops, so the value must be removed by the pattern pass.
	var b strings.Builder
	b.WriteString(`{"list":[`)
	for i := 0; i < maxSecretWalkNodes+10; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('1')
	}
	b.WriteString(`],"api_key":"FAKE-cap-0012"}`)
	client := newFakeAdmin(t, func([]byte) (int, string) {
		return http.StatusBadRequest, `refused api_key=FAKE-cap-0012`
	})

	msg := requireAdminError(t, client.LoadJSON(context.Background(), []byte(b.String())), http.StatusBadRequest)
	require.NotContains(t, msg, "FAKE-cap-0012")
	require.Contains(t, msg, "[redacted]")
}

func TestConfigSecretValuesCollectsOnlyCredentialLeaves(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"name":"plain-name",
		"provider":{"name":"cloudflare","api_token":"FAKE-cfg-0013"},
		"crowdsec":{"api_key":"FAKE-cfg-0014","retries":"FAKE-cfg-0015"},
		"token_path":"/etc/charon/FAKE-cfg-path",
		"headers":{"request":{"set":{"Cookie":["FAKE-cfg-0016"],"Accept":["FAKE-cfg-0017"]}}},
		"numbers":[1,2,3],
		"nested":{"x":{"password":"FAKE-cfg-0018"}}
	}`)

	values := configSecretValues(body)

	for _, want := range []string{"FAKE-cfg-0013", "FAKE-cfg-0014", "FAKE-cfg-0016", "FAKE-cfg-0018"} {
		require.Contains(t, values, want)
	}
	for _, unwanted := range []string{"plain-name", "cloudflare", "FAKE-cfg-0015", "FAKE-cfg-path", "FAKE-cfg-0017"} {
		require.NotContains(t, values, unwanted)
	}
}

func TestConfigSecretValuesIgnoresInvalidJSON(t *testing.T) {
	t.Parallel()
	require.Empty(t, configSecretValues([]byte(`not json`)))
	require.Empty(t, configSecretValues(nil))
}

func TestConfigSecretValuesStopsAtTheNodeCap(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString(`{"list":[`)
	for i := 0; i < maxSecretWalkNodes+10; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprint(&b, i%10)
	}
	b.WriteString(`],"api_key":"FAKE-walk-0019"}`)

	require.NotPanics(t, func() { _ = configSecretValues([]byte(b.String())) })
}

func TestFetchRedactsTheAdminAnswer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"refused password=FAKE-fetch-0020"}`))
	}))
	t.Cleanup(server.Close)
	client := newTestClient(t, server.URL)

	_, err := client.Fetch(context.Background(), "/config/")

	msg := requireAdminError(t, err, http.StatusBadRequest)
	require.NotContains(t, msg, "FAKE-fetch-0020")
	require.Contains(t, msg, "refused")
}
