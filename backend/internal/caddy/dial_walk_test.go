package caddy

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func decodeDoc(t *testing.T, raw string) any {
	t.Helper()
	var doc any
	require.NoError(t, json.Unmarshal([]byte(raw), &doc))
	return doc
}

func TestWalkDials_VisitsEveryDialAndStopsOnError(t *testing.T) {
	t.Parallel()
	doc := decodeDoc(t, `{"a":[{"dial":"10.0.0.1:1"}],"b":{"routes":[{"dial":"10.0.0.2:2"}]},"c":"x"}`)
	var seen []string
	require.NoError(t, WalkDials(doc, func(dial string) error {
		seen = append(seen, dial)
		return nil
	}))
	assert.ElementsMatch(t, []string{"10.0.0.1:1", "10.0.0.2:2"}, seen)

	stop := errors.New("stop")
	calls := 0
	err := WalkDials(doc, func(string) error {
		calls++
		return stop
	})
	assert.ErrorIs(t, err, stop)
	assert.Equal(t, 1, calls)
	assert.NoError(t, WalkDials("scalar", func(string) error { return stop }))
}

func TestHasDynamicUpstreamsAndResolver(t *testing.T) {
	t.Parallel()
	assert.True(t, HasDynamicUpstreams(decodeDoc(t, `{"x":{"dynamic_upstreams":{"source":"a"}}}`)))
	assert.False(t, HasDynamicUpstreams(decodeDoc(t, `{"upstreams":[{"dial":"10.0.0.1:1"}]}`)))
	assert.True(t, HasTransportResolver(decodeDoc(t, `[{"transport":{"resolver":{"addresses":["10.0.0.2:53"]}}}]`)))
	assert.False(t, HasTransportResolver(decodeDoc(t, `{"transport":{"protocol":"http"}}`)))
	assert.False(t, HasTransportResolver(nil))
}

// dialGuardFake refuses the dials listed in refuse and fails on the dial in failOn.
type dialGuardFake struct {
	refuse map[string]bool
	failOn string
}

func (g dialGuardFake) RefusesForwardTarget(string, int) (bool, error) { return false, nil }

func (g dialGuardFake) RefusesAdvancedDial(dial string) (bool, error) {
	if dial == g.failOn {
		return false, errors.New("check failed")
	}
	return g.refuse[dial], nil
}

// plainGuard does not implement AdvancedDialGuard, so dials are not checked.
type plainGuard struct{}

func (plainGuard) RefusesForwardTarget(string, int) (bool, error) { return false, nil }

func TestAdvancedConfigRefused_Cases(t *testing.T) {
	t.Parallel()
	guard := dialGuardFake{refuse: map[string]bool{"169.254.1.5:80": true}, failOn: "boom:1"}

	refused, err := advancedConfigRefused(guard, "")
	assert.NoError(t, err)
	assert.False(t, refused, "empty config")

	refused, err = advancedConfigRefused(guard, `{"upstreams":[{"dial":"169.254.1.5:80"}]}`)
	assert.NoError(t, err)
	assert.True(t, refused, "refused dial")

	refused, err = advancedConfigRefused(guard, `{"upstreams":[{"dial":"10.0.0.5:80"}]}`)
	assert.NoError(t, err)
	assert.False(t, refused, "allowed dial")

	refused, err = advancedConfigRefused(guard, `{"dynamic_upstreams":{}}`)
	assert.NoError(t, err)
	assert.True(t, refused, "dynamic upstreams are refused")

	refused, err = advancedConfigRefused(guard, `{"transport":{"resolver":{}}}`)
	assert.NoError(t, err)
	assert.True(t, refused, "custom resolver is refused")

	refused, err = advancedConfigRefused(guard, `{not json`)
	assert.NoError(t, err)
	assert.True(t, refused, "unreadable config is refused")

	_, err = advancedConfigRefused(guard, `{"upstreams":[{"dial":"boom:1"}]}`)
	assert.Error(t, err, "a failing check is returned")

	refused, err = advancedConfigRefused(plainGuard{}, `{"upstreams":[{"dial":"169.254.1.5:80"}]}`)
	assert.NoError(t, err)
	assert.False(t, refused, "a guard without advanced checks does not inspect dials")
}

func TestHostHasRefusedTarget_ChecksAdvancedConfig(t *testing.T) {
	t.Parallel()
	guard := dialGuardFake{refuse: map[string]bool{"169.254.1.5:80": true}}
	refused, err := hostHasRefusedTarget(guard, models.ProxyHost{
		ForwardHost:    "10.0.0.5",
		ForwardPort:    8080,
		AdvancedConfig: `{"upstreams":[{"dial":"169.254.1.5:80"}]}`,
	})
	require.NoError(t, err)
	assert.True(t, refused)
}

func TestWalkHealthCheckDials_UpstreamAndPortPairs(t *testing.T) {
	t.Parallel()
	doc := decodeDoc(t, `{"handler":"reverse_proxy",
		"upstreams":[{"dial":"10.0.0.1:8080"},{"dial":"app.lan:9000"}],
		"health_checks":{"active":{"upstream":"probe.lan:8081","port":8085}}}`)
	var seen []string
	require.NoError(t, WalkHealthCheckDials(doc, func(dial string) error {
		seen = append(seen, dial)
		return nil
	}))
	assert.ElementsMatch(t, []string{"probe.lan:8081", "10.0.0.1:8085", "app.lan:8085"}, seen)
}

func TestWalkHealthCheckDials_PortForms(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		port string
		want []string
	}{
		"integer":      {`9000`, []string{"10.0.0.1:9000"}},
		"string":       {`"9000"`, []string{"10.0.0.1:9000"}},
		"fractional":   {`9000.5`, nil},
		"zero":         {`0`, nil},
		"above range":  {`70000`, nil},
		"not a number": {`true`, nil},
		"unparsable":   {`"abc"`, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			doc := decodeDoc(t, `{"upstreams":[{"dial":"10.0.0.1:8080"}],"health_checks":{"active":{"port":`+tc.port+`}}}`)
			var seen []string
			require.NoError(t, WalkHealthCheckDials(doc, func(dial string) error {
				seen = append(seen, dial)
				return nil
			}))
			assert.Equal(t, tc.want, seen)
		})
	}
}

func TestWalkHealthCheckDials_StopsOnError(t *testing.T) {
	t.Parallel()
	doc := decodeDoc(t, `{"health_checks":{"active":{"upstream":"a:1"}}}`)
	stop := errors.New("stop")
	assert.ErrorIs(t, WalkHealthCheckDials(doc, func(string) error { return stop }), stop)
	assert.NoError(t, WalkHealthCheckDials("scalar", func(string) error { return stop }))
}

func TestWalkAdvancedDials_CoversDialsAndHealthChecks(t *testing.T) {
	t.Parallel()
	doc := decodeDoc(t, `{"upstreams":[{"dial":"10.0.0.1:8080"}],"health_checks":{"active":{"upstream":"probe.lan:8081"}}}`)
	var seen []string
	require.NoError(t, WalkAdvancedDials(doc, func(dial string) error {
		seen = append(seen, dial)
		return nil
	}))
	assert.ElementsMatch(t, []string{"10.0.0.1:8080", "probe.lan:8081"}, seen)
}

func TestWalkHealthCheckDials_DecodedNumbers(t *testing.T) {
	t.Parallel()
	dec := json.NewDecoder(strings.NewReader(`{"upstreams":[{"dial":"10.0.0.1:8080"}],"health_checks":{"active":{"port":8085}}}`))
	dec.UseNumber()
	var doc any
	require.NoError(t, dec.Decode(&doc))
	var seen []string
	require.NoError(t, WalkHealthCheckDials(doc, func(dial string) error {
		seen = append(seen, dial)
		return nil
	}))
	assert.Equal(t, []string{"10.0.0.1:8085"}, seen)
}
