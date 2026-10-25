package crowdsec

import (
	"context"
	"embed"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cdnFixtures holds a trimmed copy of the compact .index.json that the CrowdSec CDN serves for the master branch
// (the URL cscli itself uses), and the sshd collection item whose digest that index records.
//
//go:embed testdata/hub_index_cdn_compact.json testdata/hub_item_cdn_sshd.yaml
var cdnFixtures embed.FS

const (
	cdnIndexURL   = "https://cdn-hub.crowdsec.net/crowdsecurity/master/.index.json"
	cdnItemPrefix = "https://cdn-hub.crowdsec.net/crowdsecurity/master/"
	// cdnSSHDDigest is the versions["0.9"].digest recorded for collections/crowdsecurity/sshd in the fixture.
	cdnSSHDDigest = "58b871f74a370e07e950eefb3d2be9336d9ff7604b1b77f4c3fcc9c0089a69af"
)

func readCDNFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := cdnFixtures.ReadFile(filepath.ToSlash(filepath.Join("testdata", name)))
	require.NoError(t, err)
	return data
}

// The compact CDN index has no embedded content and keeps versions[<v>].digest, so it must parse through
// parseRawIndex with the CDN index URL as its base and every item must carry the digest the verifier compares.
func TestCDNCompactIndexParsesThroughParseRawIndex(t *testing.T) {
	idx, err := parseRawIndex(readCDNFixture(t, "hub_index_cdn_compact.json"), cdnIndexURL)
	require.NoError(t, err)
	require.Len(t, idx.Items, 6)

	types := make([]string, 0, len(idx.Items))
	for _, it := range idx.Items {
		types = append(types, it.Type)
		require.NotEmpty(t, it.Digest, "every fixture item records a digest for its version: %s", it.Name)
		require.True(t, strings.HasPrefix(it.DownloadURL, cdnItemPrefix), "download URLs are rooted at the CDN branch: %s", it.DownloadURL)
	}
	require.ElementsMatch(t, []string{
		"appsec-configs", "collections", "contexts", "parsers", "postoverflows", "scenarios",
	}, types, "one item of each section in the fixture")

	entry, ok := findIndexEntry(idx, "collections", "crowdsecurity/sshd")
	require.True(t, ok)
	require.Equal(t, cdnSSHDDigest, entry.Digest)
	require.Equal(t, cdnItemPrefix+"collections/crowdsecurity/sshd.yaml", entry.DownloadURL)
}

func TestCDNCompactIndexItemPassesDigestVerification(t *testing.T) {
	idx, err := parseRawIndex(readCDNFixture(t, "hub_index_cdn_compact.json"), cdnIndexURL)
	require.NoError(t, err)
	entry, ok := findIndexEntry(idx, "collections", "crowdsecurity/sshd")
	require.True(t, ok)

	item := readCDNFixture(t, "hub_item_cdn_sshd.yaml")
	require.NoError(t, verifyIndexDigest(entry, item), "the served item matches the digest the CDN index records")

	tampered := append(append([]byte(nil), item...), []byte("# appended\n")...)
	require.ErrorIs(t, verifyIndexDigest(entry, tampered), ErrHubItemUnverified, "an item that differs from the indexed digest is refused")
}

// The default index is the CDN master index, the same file cscli downloads, and the CDN answers it first.
func TestDefaultHubIndexIsCDNMaster(t *testing.T) {
	transport := &hubTransport{bodies: map[string]string{cdnIndexURL: string(readCDNFixture(t, "hub_index_cdn_compact.json"))}}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 6)
	require.Equal(t, []string{cdnIndexURL}, transport.seen(), "the CDN answers first, so no mirror or legacy host is asked")
}

func TestValidateHubURLAllowsCDNHub(t *testing.T) {
	require.NoError(t, validateHubURL(cdnIndexURL))
	require.NoError(t, validateHubURL(cdnItemPrefix+"collections/crowdsecurity/sshd.yaml"))
}

// The allowlist matches the exact hostname, so names that only contain or prefix the CDN host, a userinfo trick
// and plain http are all refused.
func TestValidateHubURLRejectsLookalikesOfCDNHost(t *testing.T) {
	t.Parallel()
	for _, rawURL := range []string{
		"https://cdn-hub.crowdsec.net.evil.example/crowdsecurity/master/.index.json",
		"https://cdn-hub.crowdsec.net@evil.example/crowdsecurity/master/.index.json",
		"https://evil.example/cdn-hub.crowdsec.net/crowdsecurity/master/.index.json",
		"https://evil-cdn-hub.crowdsec.net/crowdsecurity/master/.index.json",
		"http://cdn-hub.crowdsec.net/crowdsecurity/master/.index.json",
	} {
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()
			require.Error(t, validateHubURL(rawURL))
		})
	}
}

// Item downloads are rooted at the CDN master branch, the base cscli uses, and each item must match the digest the
// index records for it.
func TestPullDownloadsCDNItemUnderMasterAndVerifiesDigest(t *testing.T) {
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	item := string(readCDNFixture(t, "hub_item_cdn_sshd.yaml"))
	itemURL := cdnItemPrefix + "collections/crowdsecurity/sshd.yaml"
	transport := &hubTransport{bodies: map[string]string{
		cdnIndexURL: string(readCDNFixture(t, "hub_index_cdn_compact.json")),
		itemURL:     item,
	}}
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	res, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.NoError(t, err)
	require.Equal(t, item, res.Preview)
	require.Contains(t, transport.seen(), itemURL)
}

func TestPullRefusesCDNItemThatDiffersFromIndexedDigest(t *testing.T) {
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	tampered := string(readCDNFixture(t, "hub_item_cdn_sshd.yaml")) + "# appended\n"
	transport := &hubTransport{bodies: map[string]string{
		cdnIndexURL: string(readCDNFixture(t, "hub_index_cdn_compact.json")),
		cdnItemPrefix + "collections/crowdsecurity/sshd.yaml": tampered,
	}}
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	_, err = svc.Pull(context.Background(), "crowdsecurity/sshd", "collections")
	require.ErrorIs(t, err, ErrHubItemUnverified)
}

// The GitHub mirror is still the fallback when the CDN fails. This is order-agnostic, so it holds before and
// after the default changes: the CDN answers 503 and the mirror index is what gets used.
func TestFetchIndexFallsBackToGitHubMirrorWhenCDNFails(t *testing.T) {
	transport := &hubTransport{
		status: map[string]int{cdnIndexURL: http.StatusServiceUnavailable},
		bodies: map[string]string{rawIndexURL: readCSCLIFixture(t, "hub_index_raw_sample.json")},
	}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Contains(t, itemNames(idx), "crowdsecurity/sshd")
	require.Contains(t, transport.seen(), rawIndexURL, "the GitHub mirror is asked once the primary fails")
}
