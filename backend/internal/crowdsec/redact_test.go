package crowdsec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const redactGoldenDir = "testdata/redact"

// redactTokenPrefix marks every planted fixture value; fixtures never use real credentials.
const redactTokenPrefix = "SECRETTOKEN-"

func mustRedact(t *testing.T, in string) RedactResult {
	t.Helper()
	res, err := RedactNotificationFile([]byte(in))
	require.NoError(t, err)
	return res
}

func goldenInputs(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(redactGoldenDir, "*.in.txt"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	return files
}

func TestRedactGoldenFiles(t *testing.T) {
	for _, in := range goldenInputs(t) {
		name := strings.TrimSuffix(filepath.Base(in), ".in.txt")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Clean(in))
			require.NoError(t, err)
			res := mustRedact(t, string(src))
			golden := filepath.Join(redactGoldenDir, name+".golden.txt")
			if os.Getenv("CHARON_UPDATE_GOLDEN") != "" {
				require.NoError(t, os.WriteFile(golden, res.Data, 0o600))
			}
			want, err := os.ReadFile(filepath.Clean(golden))
			require.NoError(t, err)
			require.Equal(t, string(want), string(res.Data))
			require.NotContains(t, string(res.Data), redactTokenPrefix)
			require.Equal(t, strings.Count(string(res.Data), RedactedPlaceholder), res.Redacted)

			again := mustRedact(t, string(res.Data))
			require.Equal(t, string(res.Data), string(again.Data), "second pass must not change the output")
			require.Equal(t, res.Redacted, again.Redacted)
		})
	}
}

func TestRedactHeaderCommentOnlyWhenPlaceholdersExist(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: file\nname: n\nlog_path: /tmp/a\n")
	require.Zero(t, res.Redacted)
	require.NotContains(t, string(res.Data), "#")

	res = mustRedact(t, "type: splunk\nname: n\ntoken: abc\n")
	require.Equal(t, 1, res.Redacted)
	require.Equal(t, 1, strings.Count(string(res.Data), redactHeaderComment))
}

func TestRedactEmptyAndCommentOnlyStreams(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "# only a comment\n", "   \n"} {
		res := mustRedact(t, in)
		require.Empty(t, res.Data)
		require.Zero(t, res.Redacted)
	}
}

func TestRedactAlreadyRedactedFileIsByteIdentical(t *testing.T) {
	t.Parallel()
	src := "type: http\nname: n\nurl: https://hooks.example.invalid/SECRETTOKEN-aaaaaaaa\nheaders:\n  Authorization: Bearer SECRETTOKEN-bbbbbbbb\n"
	first := mustRedact(t, src)
	second := mustRedact(t, string(first.Data))
	require.Equal(t, string(first.Data), string(second.Data))
	require.Equal(t, 2, second.Redacted)
	require.Equal(t, 1, strings.Count(string(second.Data), redactHeaderComment))
}

func TestRedactKeepsStructureForEachPlugin(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: email\nname: n\nsmtp_host: smtp.example.invalid\nsmtp_password: pw\nreceiver_emails:\n  - a@example.invalid\n")
	docs, err := ParseYAMLDocuments(bytes.NewReader(res.Data), false)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, docs[0].Decode(&out))
	require.Equal(t, "email", out["type"])
	require.Equal(t, "n", out["name"])
	require.Equal(t, "smtp.example.invalid", out["smtp_host"])
	require.Equal(t, RedactedPlaceholder, out["smtp_password"])
	require.Equal(t, []any{"a@example.invalid"}, out["receiver_emails"])
}

func TestRedactNamesAndTypesSurvive(t *testing.T) {
	t.Parallel()
	for _, in := range goldenInputs(t) {
		src, err := os.ReadFile(filepath.Clean(in))
		require.NoError(t, err)
		res := mustRedact(t, string(src))
		require.Equal(t, definedPairs(t, src), definedPairs(t, res.Data), in)
	}
}

func definedPairs(t *testing.T, data []byte) []string {
	t.Helper()
	docs, err := ParseYAMLDocuments(bytes.NewReader(data), false)
	require.NoError(t, err)
	var pairs []string
	for _, d := range docs {
		var m map[string]any
		if d.Decode(&m) != nil || m == nil {
			continue
		}
		if typ, ok := m["type"].(string); ok {
			name, _ := m["name"].(string)
			pairs = append(pairs, typ+"/"+name)
		}
	}
	return pairs
}

func TestRedactUnsafeInputsBecomeErrors(t *testing.T) {
	t.Parallel()
	expanding := "a: &a [x,x,x,x,x,x,x,x,x]\n"
	prev := "a"
	for _, n := range []string{"b", "c", "d", "e", "f", "g", "h", "i", "j", "k"} {
		expanding += fmt.Sprintf("%s: &%s [*%s,*%s,*%s,*%s,*%s,*%s,*%s,*%s,*%s]\n", n, n, prev, prev, prev, prev, prev, prev, prev, prev, prev)
		prev = n
	}
	cases := map[string]string{
		"duplicate keys":      "type: http\nurl: a\nurl: b\n",
		"alias expansion":     expanding,
		"complex key":         "? [a]\n: b\n",
		"nul byte":            "type: http\nurl: a\x00b\n",
		"invalid utf8":        "type: http\nurl: \xff\xfe\n",
		"binary":              string([]byte{0x01, 0x02, 0x03, 0xff, 0x00}),
		"unterminated":        "type: http\nurl: [a\n",
		"oversize":            "type: file\n# " + strings.Repeat("a", int(maxInspectEntryBytes)) + "\n",
		"too many nodes":      "a: [" + strings.Repeat("1,", maxRedactNodes) + "1]\n",
		"unknown anchor":      "type: http\nurl: *nope\n",
		"anchor in collapsed": "type: slack\nunk: {inner: &S SECRETTOKEN-detached}\nchannel: *S\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res, err := RedactNotificationFile([]byte(in))
			require.ErrorIs(t, err, ErrRedactUnsafe)
			require.Empty(t, res.Data)
			require.NotContains(t, err.Error(), redactTokenPrefix)
			require.NotContains(t, err.Error(), "detached")
		})
	}
}

func TestRedactDetachedAnchorStubCarriesNothing(t *testing.T) {
	t.Parallel()
	_, err := RedactNotificationFile([]byte("type: slack\nunk: {inner: &S TOKEN}\nchannel: *S\n"))
	require.ErrorIs(t, err, ErrRedactUnsafe)
	require.NotContains(t, err.Error(), "TOKEN")
	require.NotContains(t, err.Error(), "S ")

	res := mustRedact(t, "type: http\nunk: {inner: &S TOKEN}\nchannel: *S\n")
	require.NotContains(t, string(res.Data), "TOKEN")
	require.NotContains(t, string(res.Data), "&S")
	require.NotContains(t, string(res.Data), "*S")
	require.Equal(t, 2, res.Redacted)
}

func TestRedactStubContentHoldsNoDocument(t *testing.T) {
	t.Parallel()
	docs, err := ParseYAMLDocuments(strings.NewReader(RedactStubContent), false)
	require.NoError(t, err)
	require.Empty(t, docs)
}

func TestRedactFailsClosedOnShapes(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: http\nname: n\nheaders: SECRETTOKEN-text\nmethod: [POST]\ntimeout: &t 5s\nlog_level: *t\n")
	require.NotContains(t, string(res.Data), redactTokenPrefix)
	require.Contains(t, string(res.Data), "headers: "+RedactedPlaceholder)
	require.Contains(t, string(res.Data), "method: "+RedactedPlaceholder)
	require.Contains(t, string(res.Data), "log_level: *a1")

	res = mustRedact(t, "type: http\nname: n\nmethod: &m\n  a: SECRETTOKEN-one\nlog_level: *m\n")
	require.NotContains(t, string(res.Data), redactTokenPrefix)
	require.Contains(t, string(res.Data), "log_level: *a1")
}

func TestRedactTypeMustBeDirectPlainString(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]string{
		"quoted":  "type: \"slack\"\nwebhook: SECRETTOKEN-x\nchannel: SECRETTOKEN-chan\n",
		"merged":  "base: &b {type: slack}\n<<: *b\nwebhook: SECRETTOKEN-x\nchannel: SECRETTOKEN-chan\n",
		"aliased": "t: &t slack\ntype: *t\nwebhook: SECRETTOKEN-x\nchannel: SECRETTOKEN-chan\n",
		"upper":   "type: SLACK\nwebhook: SECRETTOKEN-x\nchannel: SECRETTOKEN-chan\n",
		"binary":  "type: !!binary c2xhY2s=\nwebhook: SECRETTOKEN-x\nchannel: SECRETTOKEN-chan\n",
		"list":    "type: [slack]\nwebhook: SECRETTOKEN-x\nchannel: SECRETTOKEN-chan\n",
		"missing": "webhook: SECRETTOKEN-x\nchannel: SECRETTOKEN-chan\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res, err := RedactNotificationFile([]byte(in))
			if err == nil {
				require.NotContains(t, string(res.Data), redactTokenPrefix)
			} else {
				require.ErrorIs(t, err, ErrRedactUnsafe)
			}
		})
	}
}

func TestRedactMergeKeyVariants(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]string{
		"alias to mapping":     "type: http\nname: n\nbase: &b\n  url: SECRETTOKEN-merge-url\n  method: PUT\n<<: *b\n",
		"inline mapping":       "type: http\nname: n\n<<: {url: SECRETTOKEN-inline-url}\n",
		"sequence of aliases":  "type: http\nname: n\na: &a {url: SECRETTOKEN-a-url}\nb: &b {headers: {X: SECRETTOKEN-b-hdr}}\n<<: [*a, *b]\n",
		"sequence with scalar": "type: http\nname: n\na: &a {url: SECRETTOKEN-a-url}\n<<: [*a]\n",
		"alias to sequence":    "type: http\nname: n\nl: &l [SECRETTOKEN-list]\n<<: *l\n",
		"scalar merge value":   "type: http\nname: n\n<<: SECRETTOKEN-scalar\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res, err := RedactNotificationFile([]byte(in))
			if err == nil {
				require.NotContains(t, string(res.Data), redactTokenPrefix)
				require.NotContains(t, string(res.Data), "!!merge")
			} else {
				require.ErrorIs(t, err, ErrRedactUnsafe)
			}
		})
	}
	res := mustRedact(t, "type: http\nname: n\nheaders: &h {url: SECRETTOKEN-merge-url, method: PUT}\n<<: *h\n")
	require.NotContains(t, string(res.Data), "SECRETTOKEN")
	require.Contains(t, string(res.Data), "url: "+RedactedPlaceholder)
}

func TestRedactMergeKeyPrintsWithoutTag(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: http\nname: n\nheaders: &h\n  A: ${A}\n<<: *h\n")
	require.Contains(t, string(res.Data), "<<: *a1")
	require.NotContains(t, string(res.Data), "!!")
}

func TestRedactKeyTextLimitIsPinned(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: file\nname: n\nlog_path: /a\nUnknownRoot: SECRETTOKEN-v1\nrotate:\n  UnknownChild: SECRETTOKEN-v2\n---\ntype: http\nname: n\nheaders:\n  Header-Name: SECRETTOKEN-v3\n")
	out := string(res.Data)
	require.Contains(t, out, "UnknownRoot: "+RedactedPlaceholder)
	require.Contains(t, out, "UnknownChild: "+RedactedPlaceholder)
	require.Contains(t, out, "Header-Name: "+RedactedPlaceholder)
	require.NotContains(t, out, "SECRETTOKEN")
}

func TestRedactEnvReferencesStayLiteralTextElsewhereDoesNot(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: http\nname: n\nurl: ${URL}\nheaders:\n  A: Bearer ${T}\n  B: https://h.example.invalid/${X}/abc\n")
	out := string(res.Data)
	require.Contains(t, out, "url: ${URL}")
	require.Contains(t, out, "A: Bearer ${T}")
	require.Contains(t, out, "B: "+RedactedPlaceholder)
}

func TestRedactBoolLikeScalarsAreReplaced(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: sentinel\nname: n\nshared_key: yes\ncustomer_id: off\nextra: ~\nmore: null\nlog_type: on\nempty:\n")
	out := string(res.Data)
	for _, key := range []string{"shared_key", "customer_id", "extra", "more"} {
		require.Contains(t, out, key+": "+RedactedPlaceholder)
	}
	require.Contains(t, out, "log_type: on")
	require.Contains(t, out, "empty:")
	require.NotContains(t, out, "empty: "+RedactedPlaceholder)
}

func TestRedactScrubsCommentsTagsAndAnchorNames(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "# SECRETTOKEN-doc\ntype: http # SECRETTOKEN-line\nname: !SECRETTOKEN-tag n\nheaders: &SECRETTOKEN-anchor\n  A: ${A}\n  B: &SECRETTOKEN-second ${B}\nrotate: *SECRETTOKEN-anchor\n# SECRETTOKEN-foot\n")
	out := string(res.Data)
	require.NotContains(t, out, "SECRETTOKEN")
	require.Contains(t, out, "headers: &a1")
	require.Contains(t, out, "&a2")
	require.NotContains(t, out, "!")
	require.Equal(t, 1, strings.Count(out, "#"))
}

func TestRedactAnchorNumberingRestartsPerDocument(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: http\nname: a\nheaders: &x\n  A: ${A}\n---\ntype: http\nname: b\nheaders: &y\n  A: ${A}\n")
	require.Equal(t, 2, strings.Count(string(res.Data), "&a1"))
}

func TestRedactAcceptsPlaceholderInDifferentStyles(t *testing.T) {
	t.Parallel()
	res := mustRedact(t, "type: splunk\nname: n\nurl: \"<charon-redacted>\"\ntoken: '<charon-redacted>'\n")
	require.Equal(t, 2, res.Redacted)
	again := mustRedact(t, string(res.Data))
	require.Equal(t, string(res.Data), string(again.Data))
}

func TestRedactCanary(t *testing.T) {
	t.Parallel()
	t.Run("kept duplicate of a replaced value is not a leak", func(t *testing.T) {
		t.Parallel()
		res := mustRedact(t, "type: email\nname: n\nsender_email: alerts@example.invalid\nsmtp_username: alerts@example.invalid\nsmtp_password: SECRETTOKEN-pw\n")
		require.Contains(t, string(res.Data), "sender_email: alerts@example.invalid")
		require.Contains(t, string(res.Data), "smtp_username: "+RedactedPlaceholder)
	})
	t.Run("replaced value inside a kept scalar yields an error", func(t *testing.T) {
		t.Parallel()
		_, err := RedactNotificationFile([]byte("type: email\nname: n\nsmtp_password: hunter2hunter2\nemail_subject: alert hunter2hunter2 now\n"))
		require.ErrorIs(t, err, ErrRedactUnsafe)
	})
	t.Run("collapsed subtree value inside a kept scalar yields an error", func(t *testing.T) {
		t.Parallel()
		_, err := RedactNotificationFile([]byte("type: http\nname: n\nfuture:\n  k: hunter2hunter2\nformat: \"{{ hunter2hunter2 }}\"\n"))
		require.ErrorIs(t, err, ErrRedactUnsafe)
	})
	t.Run("short values are only replaced", func(t *testing.T) {
		t.Parallel()
		res := mustRedact(t, "type: http\nname: n\nurl: abc\nformat: abc\n")
		require.Contains(t, string(res.Data), "url: "+RedactedPlaceholder)
	})
	t.Run("placeholder and header text never trigger the scan", func(t *testing.T) {
		t.Parallel()
		src := "# " + redactHeaderComment + "\ntype: http\nname: n\nurl: <charon-redacted>\nformat: <charon-redacted> <charon-redacted>\n"
		res := mustRedact(t, src)
		require.Equal(t, 1, res.Redacted)
	})
	t.Run("work bound", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		b.WriteString("type: http\nname: n\nformat: " + strings.Repeat("x", 900_000) + "\nfuture:\n")
		for i := 0; i < 400; i++ {
			fmt.Fprintf(&b, "  k%03d: value-%05d\n", i, i)
		}
		_, err := RedactNotificationFile([]byte(b.String()))
		require.ErrorIs(t, err, ErrRedactUnsafe)
	})
}

func TestIsPlaceholder(t *testing.T) {
	t.Parallel()
	require.True(t, IsPlaceholder(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: RedactedPlaceholder}))
	require.False(t, IsPlaceholder(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "x"}))
	require.False(t, IsPlaceholder(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!binary", Value: RedactedPlaceholder}))
	require.False(t, IsPlaceholder(&yaml.Node{Kind: yaml.MappingNode, Tag: "!!str", Value: RedactedPlaceholder}))
	require.False(t, IsPlaceholder(nil))
}

func TestRedactDoesNotLeakInputInErrors(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		"type: http\nurl: SECRETTOKEN-dup\nurl: SECRETTOKEN-dup2\n",
		"type: http\nurl: *SECRETTOKEN-anchor\n",
		"type: http\nurl: [SECRETTOKEN-open\n",
	} {
		_, err := RedactNotificationFile([]byte(in))
		require.ErrorIs(t, err, ErrRedactUnsafe)
		require.NotContains(t, err.Error(), "SECRETTOKEN")
	}
}

// plantedChannels lists, per plugin type, the keys whose values are credentials.
var plantedChannels = map[string][]string{
	"http":     {"url", "unknown_option"},
	"slack":    {"webhook", "icon_url"},
	"splunk":   {"url", "token"},
	"email":    {"smtp_password", "smtp_username"},
	"sentinel": {"shared_key", "customer_id"},
	"file":     {"extra"},
	"dummy":    {"extra"},
	"other":    {"url", "token"},
}

func TestRedactPlantedTokensNeverSurvive(t *testing.T) {
	t.Parallel()
	rng := &testRand{state: 1551}
	boolLike := []string{"yes", "on", "null", "~", "off", "no"}
	n := 0
	token := func() string { n++; return fmt.Sprintf("%s%04d-%d", redactTokenPrefix, n, rng.Intn(1000)) }
	for plugin, keys := range plantedChannels {
		for round := 0; round < 40; round++ {
			var tokens []string
			plant := func() string { v := token(); tokens = append(tokens, v); return v }
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n", plant())
			fmt.Fprintf(&b, "type: %s # %s\nname: n%d\n", plugin, plant(), round)
			for _, k := range keys {
				switch rng.Intn(6) {
				case 0:
					fmt.Fprintf(&b, "%s: %s\n", k, plant())
				case 1:
					fmt.Fprintf(&b, "%s: &%s %s\n", k, plant(), plant())
				case 2:
					fmt.Fprintf(&b, "%s: !%s %s\n", k, plant(), plant())
				case 3:
					fmt.Fprintf(&b, "%s:\n  %s: %s\n  list: [%s, %s]\n", k, plant(), plant(), plant(), boolLike[rng.Intn(len(boolLike))])
				case 4:
					fmt.Fprintf(&b, "%s: %s # %s\n", k, boolLike[rng.Intn(len(boolLike))], plant())
				default:
					fmt.Fprintf(&b, "%s: |\n  %s\n  second line\n", k, plant())
				}
			}
			if plugin == "http" {
				fmt.Fprintf(&b, "headers:\n  Authorization: Bearer %s\n  X-Api: %s # %s\n", plant(), plant(), plant())
			}
			if plugin == "file" {
				fmt.Fprintf(&b, "rotate:\n  enabled: true\n  other: %s\n", plant())
			}
			fmt.Fprintf(&b, "# %s\n", plant())
			res, err := RedactNotificationFile([]byte(b.String()))
			if err != nil {
				require.ErrorIs(t, err, ErrRedactUnsafe)
				continue
			}
			for _, tok := range tokens {
				require.NotContains(t, string(res.Data), tok, "plugin %s round %d\n%s", plugin, round, b.String())
			}
			_, perr := ParseYAMLDocuments(bytes.NewReader(res.Data), false)
			require.NoError(t, perr)
		}
	}
}

func FuzzRedactNotificationFile(f *testing.F) {
	for _, in := range goldenInputsForFuzz(f) {
		f.Add(in)
	}
	f.Add([]byte("type: slack\nunk: {inner: &S TOKEN}\nchannel: *S\n"))
	f.Add([]byte("type: http\nheaders: &h\n  A: SECRETTOKEN-x\n<<: *h\n"))
	f.Add([]byte("a: &a [x]\nb: &b [*a,*a]\n"))
	f.Fuzz(func(t *testing.T, src []byte) {
		res, err := RedactNotificationFile(src)
		if err != nil {
			require.ErrorIs(t, err, ErrRedactUnsafe)
			require.NotContains(t, err.Error(), redactTokenPrefix)
			return
		}
		_, perr := ParseYAMLDocuments(bytes.NewReader(res.Data), false)
		require.NoError(t, perr)
		if !bytes.Contains(src, []byte(redactTokenPrefix)) {
			return
		}
		// A planted token may only reach the output in a kept position; those hold the same text in
		// the input under a key of the allowlist, never inside a comment, tag or anchor name.
		docs, _ := ParseYAMLDocuments(bytes.NewReader(res.Data), false)
		for _, tok := range plantedTokens(src) {
			for _, d := range docs {
				require.Empty(t, tokenLeak(d, false, tok), "planted token outside a kept position")
			}
		}
	})
}

func goldenInputsForFuzz(f *testing.F) [][]byte {
	f.Helper()
	files, err := filepath.Glob(filepath.Join(redactGoldenDir, "*.in.txt"))
	require.NoError(f, err)
	var out [][]byte
	for _, p := range files {
		b, err := os.ReadFile(filepath.Clean(p))
		require.NoError(f, err)
		out = append(out, b)
	}
	return out
}

func plantedTokens(src []byte) []string {
	var toks []string
	for _, field := range strings.FieldsFunc(string(src), func(r rune) bool {
		return r != '-' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	}) {
		if strings.HasPrefix(field, redactTokenPrefix) {
			toks = append(toks, field)
		}
	}
	return toks
}

// keptKeyNames are the keys whose values stay in a redacted file, over all plugin types.
var keptKeyNames = map[string]bool{
	"type": true, "name": true, "log_level": true, "group_wait": true, "group_threshold": true,
	"max_retry": true, "timeout": true, "format": true, "unix_socket": true, "skip_tls_verification": true,
	"method": true, "cert_path": true, "key_path": true, "ca_cert_path": true, "channel": true,
	"username": true, "icon_emoji": true, "smtp_host": true, "smtp_port": true, "sender_email": true,
	"sender_name": true, "receiver_emails": true, "email_subject": true, "encryption_type": true,
	"auth_type": true, "helo_host": true, "connect_timeout": true, "send_timeout": true, "log_type": true,
	"log_path": true, "output_file": true, "enabled": true, "max_size": true, "max_files": true,
	"max_age": true, "compress": true,
}

// tokenLeak returns a description of where tok appears in n other than in the text of a key or in the
// value of a kept key, or "" when it does not. Key text of unknown keys is kept by design.
func tokenLeak(n *yaml.Node, kept bool, tok string) string {
	for _, s := range []string{n.Anchor, n.Tag, n.HeadComment, n.LineComment, n.FootComment} {
		if strings.Contains(s, tok) {
			return "metadata"
		}
	}
	if n.Kind == yaml.ScalarNode && !kept && strings.Contains(n.Value, tok) {
		return "value"
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			for _, s := range []string{key.Anchor, key.Tag, key.HeadComment, key.LineComment, key.FootComment} {
				if strings.Contains(s, tok) {
					return "key metadata"
				}
			}
			if leak := tokenLeak(n.Content[i+1], keptKeyNames[key.Value], tok); leak != "" {
				return leak
			}
		}
		return ""
	}
	for _, c := range n.Content {
		if leak := tokenLeak(c, kept, tok); leak != "" {
			return leak
		}
	}
	return ""
}

// testRand is a tiny deterministic generator so the planted-token test is reproducible.
type testRand struct{ state uint64 }

func (r *testRand) Intn(n int) int {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return int((r.state >> 33) % uint64(n)) //nolint:gosec // G115: n is a small positive slice length in tests
}
