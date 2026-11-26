package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameUpstreamWithoutResolver(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		raw  string
		want bool
	}{
		"name with no transport uses the resolver": {
			`{"handler":"reverse_proxy","upstreams":[{"dial":"app.lan:8080"}]}`, false,
		},
		"name with http and tls uses the resolver": {
			`{"handler":"reverse_proxy","transport":{"protocol":"http","tls":{}},"upstreams":[{"dial":"app.lan:8080"}]}`, false,
		},
		"name with http 1.1 and 2 versions uses the resolver": {
			`{"handler":"reverse_proxy","transport":{"protocol":"http","versions":["1.1","2"]},"upstreams":[{"dial":"app.lan:8080"}]}`, false,
		},
		"name with null network proxy uses the resolver": {
			`{"handler":"reverse_proxy","transport":{"protocol":"http","network_proxy":null},"upstreams":[{"dial":"app.lan:8080"}]}`, false,
		},
		"name with fastcgi cannot use the resolver": {
			`{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"app.lan:9000"}]}`, true,
		},
		"name with http 3 cannot use the resolver": {
			`{"handler":"reverse_proxy","transport":{"protocol":"http","versions":["3"]},"upstreams":[{"dial":"app.lan:8080"}]}`, true,
		},
		"name with a network proxy cannot use the resolver": {
			`{"handler":"reverse_proxy","transport":{"protocol":"http","network_proxy":{"module":"url"}},"upstreams":[{"dial":"app.lan:8080"}]}`, true,
		},
		"name with a forward proxy url cannot use the resolver": {
			`{"handler":"reverse_proxy","transport":{"protocol":"http","forward_proxy_url":"http://p:3128"},"upstreams":[{"dial":"app.lan:8080"}]}`, true,
		},
		"non-object transport cannot use the resolver": {
			`{"handler":"reverse_proxy","transport":"x","upstreams":[{"dial":"app.lan:8080"}]}`, true,
		},
		"literal upstream with fastcgi is allowed": {
			`{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"10.0.0.9:9000"}]}`, false,
		},
		"literal upstream with fastcgi and a name health check": {
			`{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"10.0.0.9:9000"}],"health_checks":{"active":{"upstream":"probe.lan:9000"}}}`, true,
		},
		"literal upstream with fastcgi and a literal health check": {
			`{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"10.0.0.9:9000"}],"health_checks":{"active":{"upstream":"10.0.0.9:9001"}}}`, false,
		},
		"unparsable dials are not names": {
			`{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"{http.request.header.X}:80"}]}`, false,
		},
		"nested subroute is checked": {
			`{"handler":"subroute","routes":[{"handle":[{"handler":"reverse_proxy","transport":{"protocol":"fastcgi"},"upstreams":[{"dial":"app.lan:9000"}]}]}]}`, true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, NameUpstreamWithoutResolver(decodeDoc(t, tc.raw)))
		})
	}
}
