package crowdsec

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const progressLine = "Loaded: 168 parsers, 13 postoverflows, 788 scenarios, 10 contexts, 20 appsec-configs, 229 appsec-rules, 172 collections\n"

func TestExtractCSCLIJSON(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   string
		want string
		ok   bool
	}{
		"plain object":          {in: `{"a":1}`, want: `{"a":1}`, ok: true},
		"plain array":           {in: `[1,2]`, want: `[1,2]`, ok: true},
		"progress line first":   {in: progressLine + "{\"a\":1}\n", want: `{"a":1}`, ok: true},
		"several lines first":   {in: "level=warning msg=\"x\"\n" + progressLine + "[1]\n", want: `[1]`, ok: true},
		"text after document":   {in: "{\"a\":1}\nUnmanaged items: 0 local\n", want: `{"a":1}`, ok: true},
		"brace inside a line":   {in: "note {not json}\n{\"a\":1}", want: `{"a":1}`, ok: true},
		"invalid start skipped": {in: "{broken\n{\"a\":1}", want: `{"a":1}`, ok: true},
		"no document":           {in: progressLine, ok: false},
		"empty":                 {in: "", ok: false},
		"truncated":             {in: progressLine + `{"a":`, ok: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := extractCSCLIJSON([]byte(tc.in))
			if !tc.ok {
				require.ErrorIs(t, err, errCSCLIOutput)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestExtractCSCLIJSONBounds(t *testing.T) {
	t.Parallel()
	_, err := extractCSCLIJSON(make([]byte, maxCSCLIOutput+1))
	require.ErrorIs(t, err, ErrCSCLIOutputTooLarge)

	// Many broken starts are not all tried.
	_, err = extractCSCLIJSON([]byte(strings.Repeat("{x\n", 1000)))
	require.ErrorIs(t, err, errCSCLIOutput)
}

func TestUnmarshalCSCLIJSONErrorsDoNotEchoOutput(t *testing.T) {
	t.Parallel()
	var v struct {
		A int `json:"a"`
	}
	err := unmarshalCSCLIJSON([]byte("secret-token\n"), &v)
	require.EqualError(t, err, "unexpected cscli output")

	err = unmarshalCSCLIJSON([]byte(`{"a":"secret-token"}`), &v)
	require.EqualError(t, err, "unexpected cscli output")
}

func TestInspectParsesOutputWithProgressLine(t *testing.T) {
	t.Parallel()
	exec := &fixedExecutor{out: []byte(readCSCLIFixture(t, "inspect_with_progress.txt"))}

	require.NoError(t, VerifyHubItem(context.Background(), exec, "collections", "crowdsecurity/nginx"))

	svc := &HubService{Exec: exec}
	state, err := svc.inspectItem(context.Background(), "collections", "crowdsecurity/nginx")
	require.NoError(t, err)
	require.True(t, *state.Installed)
	require.False(t, state.Tainted)
	require.NotEmpty(t, state.LocalHash)
}

func TestInspectRejectsOutputWithoutDocument(t *testing.T) {
	t.Parallel()
	exec := &fixedExecutor{out: []byte(progressLine)}
	err := VerifyHubItem(context.Background(), exec, "collections", "crowdsecurity/nginx")
	require.ErrorIs(t, err, errCSCLIOutput)
	require.NotContains(t, err.Error(), "Loaded")

	_, err = (&HubService{Exec: exec}).inspectItem(context.Background(), "collections", "crowdsecurity/nginx")
	require.ErrorIs(t, err, errCSCLIOutput)
}

func TestHubListParsesOutputWithProgressLine(t *testing.T) {
	t.Parallel()
	exec := &fixedExecutor{out: []byte(readCSCLIFixture(t, "hub_list_with_progress.txt"))}
	svc := &HubService{Exec: exec}

	status, err := svc.hubListStatus(context.Background())
	require.NoError(t, err)
	require.Equal(t, "enabled", status["collections/crowdsecurity/nginx"])
	require.Equal(t, "enabled", status["parsers/crowdsecurity/nginx-logs"])
	require.Empty(t, taintedItems(status, map[string]bool{"collections/crowdsecurity/nginx": true}))

	item := []byte("name: crowdsecurity/nginx\nparsers:\n  - crowdsecurity/nginx-logs\n")
	warnings, err := svc.checkDependencies(context.Background(), CachedPreset{Type: "collections", Name: "crowdsecurity/nginx"}, item)
	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestHubListRejectsOutputWithoutDocument(t *testing.T) {
	t.Parallel()
	_, err := (&HubService{Exec: &fixedExecutor{out: []byte(progressLine)}}).hubListStatus(context.Background())
	require.ErrorIs(t, err, errCSCLIOutput)
}

func TestParseCSCLIIndexWithProgressLine(t *testing.T) {
	t.Parallel()
	raw := append([]byte(progressLine), []byte(readCSCLIFixture(t, "hub_list_all.json"))...)
	idx, err := parseCSCLIIndex(raw)
	require.NoError(t, err)
	require.NotEmpty(t, idx.Items)

	_, err = parseCSCLIIndex([]byte("cscli hub list: invalid hub index\n"))
	require.ErrorIs(t, err, errCSCLIOutput)
}

func TestFetchIndexCSCLIWithProgressLine(t *testing.T) {
	t.Parallel()
	raw := append([]byte(progressLine), []byte(readCSCLIFixture(t, "hub_list_all.json"))...)
	svc := &HubService{Exec: &fixedExecutor{out: raw}, PullTimeout: 5e9}
	idx, err := svc.fetchIndexCSCLI(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, idx.Items)
}

func TestTaintedItems(t *testing.T) {
	t.Parallel()
	got := taintedItems(map[string]string{
		"parsers/crowdsecurity/nginx-logs": "enabled,tainted",
		"collections/crowdsecurity/nginx":  "enabled,tainted",
		"scenarios/crowdsecurity/ok":       "enabled",
		"scenarios/bad name\n/x":           "enabled,tainted",
		"unknown/crowdsecurity/x":          "enabled,tainted",
		"collections":                      "enabled,tainted",
		"scenarios/crowdsecurity/..%2f..":  "enabled,tainted",
	}, map[string]bool{
		"parsers/crowdsecurity/nginx-logs": true, "collections/crowdsecurity/nginx": true, "scenarios/crowdsecurity/ok": true,
		"scenarios/bad name\n/x": true, "unknown/crowdsecurity/x": true, "collections": true, "scenarios/crowdsecurity/..%2f..": true,
	})
	require.Equal(t, []string{"collections/crowdsecurity/nginx", "parsers/crowdsecurity/nginx-logs"}, got)

	many := map[string]string{}
	for i := range maxTaintedItems + 10 {
		many["scenarios/crowdsecurity/s"+strings.Repeat("a", i%5)+string(rune('a'+i%26))+string(rune('a'+i/26))] = "enabled,tainted"
	}
	inScope := map[string]bool{}
	for k := range many {
		inScope[k] = true
	}
	require.LessOrEqual(t, len(taintedItems(many, inScope)), maxTaintedItems)
}

func TestLocalModifiedErrorUnwraps(t *testing.T) {
	t.Parallel()
	err := error(&LocalModifiedError{Items: []string{"collections/a/b"}})
	require.ErrorIs(t, err, ErrPresetLocalModified)
	require.Equal(t, ErrPresetLocalModified.Error(), err.Error())
	var target *LocalModifiedError
	require.True(t, errors.As(err, &target))
}

// fixedExecutor answers every command with the same output.
type fixedExecutor struct{ out []byte }

func (f *fixedExecutor) Execute(context.Context, string, ...string) ([]byte, error) {
	return f.out, nil
}

func TestItemMembersReadsRealInspectOutput(t *testing.T) {
	t.Parallel()
	svc := &HubService{Exec: &fixedExecutor{out: []byte(readCSCLIFixture(t, "inspect_collection_members.json"))}}
	got := svc.itemMembers(context.Background(), HubItem{Type: "collections", Name: "crowdsecurity/nginx"})
	require.ElementsMatch(t, []HubItem{
		{Type: "collections", Name: "crowdsecurity/base-http-scenarios"},
		{Type: "parsers", Name: "crowdsecurity/nginx-logs"},
		{Type: "scenarios", Name: "crowdsecurity/nginx-req-limit-exceeded"},
	}, got)
}

func TestUnmarshalCSCLIOutput_AboveTheLimit(t *testing.T) {
	var v any

	t.Run("output at the limit is read", func(t *testing.T) {
		out := make([]byte, maxCSCLIOutput)
		for i := range out {
			out[i] = ' '
		}
		copy(out, "[]\n")
		require.NoError(t, UnmarshalCSCLIOutput(out, &v))
	})

	t.Run("output above the limit has its own error", func(t *testing.T) {
		out := make([]byte, maxCSCLIOutput+1)
		copy(out, "[]\n")
		err := UnmarshalCSCLIOutput(out, &v)
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "too large", "the sentinel ErrCSCLIOutputTooLarge says the output is too large")
		assert.NotContains(t, err.Error(), "unexpected cscli output", "it is not the parse error")
	})

	t.Run("garbage within the limit is the parse error", func(t *testing.T) {
		err := UnmarshalCSCLIOutput([]byte("no json here"), &v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected cscli output")
	})
}
