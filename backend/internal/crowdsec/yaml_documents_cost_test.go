package crowdsec

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const skipYAMLCost = "enabled by a later commit in this series"

// parseTimeBound is far above a healthy parse of these inputs and far below a runaway one; it keeps the
// timing assertions stable under the race detector.
const parseTimeBound = 30 * time.Second

// expansionShape builds one mapping with flat scalars, an anchored list of anchorSize scalars and a list
// of refs aliases to it. Its expanded size is about flat + anchorSize*refs nodes while the text stays a
// few hundred kilobytes; yaml.v3's own alias check only rejects it when aliases dominate the expansion.
func expansionShape(flat, anchorSize, refs int) string {
	var b strings.Builder
	b.WriteString("anchor: &a [" + strings.TrimSuffix(strings.Repeat("0,", anchorSize), ",") + "]\n")
	b.WriteString("flat: [" + strings.TrimSuffix(strings.Repeat("1,", flat), ",") + "]\n")
	b.WriteString("refs: [" + strings.TrimSuffix(strings.Repeat("*a,", refs), ",") + "]\n")
	return b.String()
}

func timedParse(t *testing.T, input string) (time.Duration, error) {
	t.Helper()
	start := time.Now()
	_, err := ParseYAMLDocuments(strings.NewReader(input), false)
	elapsed := time.Since(start)
	t.Logf("parsed %d bytes in %s (err=%v)", len(input), elapsed, err)
	return elapsed, err
}

func TestParseYAMLDocumentsRejectsExpansionAboveTheCap(t *testing.T) {
	t.Skip(skipYAMLCost)
	input := expansionShape(300_000, 100, 9_000) // about 1.2M expanded nodes

	elapsed, err := timedParse(t, input)

	require.ErrorContains(t, err, "too complex")
	require.Less(t, elapsed, parseTimeBound)
}

func TestParseYAMLDocumentsAcceptsExpansionBelowTheCap(t *testing.T) {
	input := expansionShape(300_000, 100, 6_000) // about 900k expanded nodes

	elapsed, err := timedParse(t, input)

	require.NoError(t, err)
	require.Less(t, elapsed, parseTimeBound)
}

func TestParseYAMLDocumentsBoundsAliasHeavyInput(t *testing.T) {
	input := expansionShape(10, 100, 250_000) // about 1 MiB of text, aliases dominate

	elapsed, err := timedParse(t, input)

	require.Error(t, err)
	require.Regexp(t, "too complex|excessive aliasing", err.Error())
	require.Less(t, elapsed, parseTimeBound)
}

func TestParseYAMLDocumentsRejectsASelfReferencingAnchor(t *testing.T) {
	t.Parallel()

	elapsed, err := timedParse(t, "a: &a [*a]\n")

	require.Error(t, err)
	require.Less(t, elapsed, parseTimeBound)
}

func TestParseYAMLDocumentsKeepsBenignAnchorsAndMergeKeys(t *testing.T) {
	t.Parallel()
	input := "base: &base\n  x: 1\n  list: &items [a, b, c]\nderived:\n  <<: *base\n  y: 2\n  again: *items\n"

	docs, err := ParseYAMLDocuments(strings.NewReader(input), false)

	require.NoError(t, err)
	require.Len(t, docs, 1)
}

func TestParseYAMLDocumentsReadsAtMostTheReaderCap(t *testing.T) {
	t.Parallel()
	t.Skip(skipYAMLCost)
	padding := func(n int) string { return "k: v\n#" + strings.Repeat("x", n-len("k: v\n#")-1) + "\n" }

	_, err := ParseYAMLDocuments(strings.NewReader(padding(2<<20)), false)
	require.NoError(t, err, "a stream of exactly the cap is read")

	_, err = ParseYAMLDocuments(strings.NewReader(padding(2<<20+1)), false)
	require.ErrorContains(t, err, "too large")
}

func TestYAMLCostFailsClosedForCallers(t *testing.T) {
	t.Skip(skipYAMLCost)
	over := expansionShape(300_000, 100, 9_000)

	t.Run("configured notification folder", func(t *testing.T) {
		cfg := "config_paths:\n  notification_dir: /etc/crowdsec/custom_notify\n" + over
		_, ok := configuredNotificationDir([]byte(cfg))
		require.False(t, ok, "an over-complex config.yaml selects no folder")
	})
	t.Run("hub item", func(t *testing.T) {
		require.Error(t, checkHubItem([]byte("name: crowdsecurity/demo\n"+over)), "an over-complex item is never cached")
	})
}
