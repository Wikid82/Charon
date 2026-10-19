package crowdsec

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// testdata/hub_index_ambiguous.json is an excerpt of the raw index `cscli hub update` downloads: crowdsecurity/crs
// is an item of two sections, crowdsecurity/k8s-audit of two as well.
func TestRawIndexExcerptHoldsSameNamedItemsOfSeveralTypes(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("testdata", "hub_index_ambiguous.json"))
	require.NoError(t, err)

	idx, err := parseRawIndex(raw, "https://hub.example.invalid/.index.json")

	require.NoError(t, err)
	typesByName := map[string][]string{}
	for _, item := range idx.Items {
		typesByName[item.Name] = append(typesByName[item.Name], item.Type)
	}
	for _, types := range typesByName {
		sort.Strings(types)
	}
	require.Equal(t, []string{"appsec-configs", "appsec-rules"}, typesByName["crowdsecurity/crs"])
	require.Equal(t, []string{"collections", "parsers"}, typesByName["crowdsecurity/k8s-audit"])
	require.Equal(t, []string{"collections"}, typesByName["crowdsecurity/nginx"])
	for _, item := range idx.Items {
		require.True(t, IsCacheItemType(item.Type), item.Type)
		require.Contains(t, item.DownloadURL, "/"+item.Type+"/", "the download location carries the section")
	}
}
