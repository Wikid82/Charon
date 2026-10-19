package crowdsec

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidHubItemName(t *testing.T) {
	t.Parallel()
	good := []string{
		"crowdsecurity/sshd", "Azlaroc/sftpgo", "Dominic-Wagner/vaultwarden", "crowdsecurity/CVE-2022-26134",
		"a/b", "0/0", "o.w_n-er/na.me_1-x", strings.Repeat("a", 64) + "/" + strings.Repeat("b", 128),
	}
	for _, n := range good {
		require.True(t, ValidHubItemName(n), n)
	}
	bad := []string{
		"", "/", "a/", "/b", "-a/b", "a/-b", "--help", "-h/x", "a/b/c", "a/../b", "../x", "a..b/c", "a/b..c", "a;b/c", "a/b c",
		"a/b\n", "a\n/b", "ä/b", "a/ö", ".a/b", "a/.b", "a/b$", "a/`b`", "a b/c", "nofolder",
		strings.Repeat("a", 65) + "/b", "a/" + strings.Repeat("b", 129),
	}
	for _, n := range bad {
		require.False(t, ValidHubItemName(n), "%q", n)
	}
}

func TestValidPresetItemNameIsStricter(t *testing.T) {
	t.Parallel()
	require.True(t, ValidPresetItemName("crowdsecurity/sshd"))
	require.True(t, ValidPresetItemName("a_b/c.d-e"))
	require.False(t, ValidPresetItemName("CrowdSecurity/sshd"))
	require.False(t, ValidPresetItemName("crowdsecurity/CVE-2022-26134"))
	require.False(t, ValidPresetItemName("a/../b"))
	require.False(t, ValidPresetItemName("a/b\n"))
}

func TestHubItemTypes(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"collections", "parsers", "scenarios", "postoverflows", "contexts"} {
		require.True(t, IsHubItemType(typ), typ)
		require.True(t, HubItem{Type: typ, Name: "a/b"}.Valid())
	}
	for _, typ := range []string{"", "appsec-rules", "appsec-configs", "Collections", "collection", "parsers/"} {
		require.False(t, IsHubItemType(typ), typ)
		require.False(t, HubItem{Type: typ, Name: "a/b"}.Valid())
	}
	require.False(t, HubItem{Type: "parsers", Name: "bad"}.Valid())
	// The restore's set is wider than the preset set; presets stay unchanged.
	require.NotContains(t, allowedPresetItemTypes, "contexts")
	require.NotContains(t, allowedPresetItemTypes, "appsec-rules")
}

type verifyExec struct {
	out  string
	err  error
	args []string
}

func (v *verifyExec) Execute(_ context.Context, name string, args ...string) ([]byte, error) {
	v.args = append([]string{name}, args...)
	return []byte(v.out), v.err
}

func TestVerifyHubItem(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		out     string
		execErr error
		wantErr string
	}{
		{"installed", `{"installed":true,"tainted":false}`, nil, ""},
		{"not installed", `{"installed":false}`, nil, "not installed"},
		{"tainted", `{"installed":true,"tainted":true}`, nil, "tainted"},
		{"no field", `{}`, nil, "no installed field"},
		{"bad json", `nope`, nil, "parse inspect output"},
		{"exec error", ``, errors.New("boom"), "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := &verifyExec{out: tc.out, err: tc.execErr}
			err := VerifyHubItem(t.Context(), exec, "", "collections", "crowdsecurity/sshd")
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
			require.Equal(t, []string{"cscli", "collections", "inspect", "crowdsecurity/sshd", "-o", "json"}, exec.args)
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	exec := &verifyExec{}
	require.ErrorIs(t, VerifyHubItem(ctx, exec, "", "collections", "a/b"), context.Canceled)
	require.Nil(t, exec.args, "a cancelled context never reaches cscli")
}
