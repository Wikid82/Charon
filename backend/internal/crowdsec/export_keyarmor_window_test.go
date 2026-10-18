package crowdsec

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	sniffHead  = 64 << 10 // the first window of the armor search
	inspectCap = 1 << 20  // files up to this size are searched completely
)

// armorAt returns a file of the given size whose private key header starts at offset. The rest is filler, so the
// header is cut by whatever window boundary lies near offset.
func armorAt(offset, size int) string {
	header := "-----BEGIN " + "RSA PRIVATE KEY-----\n"
	body := strings.Repeat("#", offset) + header
	if len(body) < size {
		body += strings.Repeat("#", size-len(body))
	}
	return body
}

// A key header that straddles the end of the first 64 KiB window, or any later window, is still found in files
// up to the 1 MiB inspect cap.
func TestExportKeyArmorIsFoundAcrossWindowBoundaries(t *testing.T) {
	t.Parallel()
	const headerLen = 31
	cases := map[string]string{}
	for _, inside := range []int{1, 15, headerLen - 1} { // header bytes before the boundary
		cases[fmt.Sprintf("header cut after %d of its bytes", inside)] = armorAt(sniffHead-inside, sniffHead+4096)
	}
	for _, window := range []int{2, 3, 8, 15} {
		cases[fmt.Sprintf("header cut near %d windows", window)] = armorAt(window*sniffHead-10, window*sniffHead+4096)
		cases[fmt.Sprintf("header cut near %d windows less the overlap", window)] = armorAt(window*(sniffHead-128)-10, window*sniffHead+4096)
	}
	cases["armor in the middle of a 600 KiB file"] = armorAt(300<<10, 600<<10)
	cases["armor at the very end of a file of exactly the cap"] = armorAt(inspectCap-32, inspectCap)

	files := map[string]string{"config/ssl/kept.txt": strings.Repeat("#", 200<<10)}
	names := map[string]string{}
	i := 0
	for label, content := range cases {
		name := fmt.Sprintf("config/ssl/case%02d.txt", i)
		i++
		files[name], names[name] = content, label
	}
	entries := exportedKeyTree(t, files)

	for name, label := range names {
		require.NotContains(t, entries, name, label)
	}
	require.Contains(t, entries, "config/ssl/kept.txt", "filler alone is not a key")
}

// A header that ends exactly at the window end is found today; one that starts right after it is found once the
// whole file below the cap is searched.
func TestExportKeyArmorNextToTheFirstBoundaryIsFound(t *testing.T) {
	t.Parallel()
	entries := exportedKeyTree(t, map[string]string{
		"config/ssl/inside.txt": armorAt(sniffHead-31, sniffHead+100),
		"config/ssl/after.txt":  armorAt(sniffHead, sniffHead+100),
	})

	require.NotContains(t, entries, "config/ssl/inside.txt")
	require.NotContains(t, entries, "config/ssl/after.txt", "files below the cap are searched completely")
}

// Above the inspect cap only the first 64 KiB are read (documented limit), so a late header stays unseen and an
// early one is still found.
func TestExportKeyArmorInFilesAboveTheCapReadsOnlyTheHead(t *testing.T) {
	t.Parallel()
	entries := exportedKeyTree(t, map[string]string{
		"config/ssl/huge-late.txt":  armorAt(500<<10, inspectCap+1024),
		"config/ssl/huge-early.txt": armorAt(100, inspectCap+1024),
	})

	require.Contains(t, entries, "config/ssl/huge-late.txt", "beyond the cap only the head is searched")
	require.NotContains(t, entries, "config/ssl/huge-early.txt")
}

// The engine's own config files are never searched, so a comment that mentions a header stays exported.
func TestExportKeyArmorSearchSkipsEngineFilesBeyondTheFirstWindow(t *testing.T) {
	t.Parallel()
	entries := exportedKeyTree(t, map[string]string{
		"config/profiles.yaml": "name: default\n# " + strings.Repeat("x", 200<<10) + "\n# -----BEGIN RSA PRIVATE KEY-----\n",
	})

	require.Contains(t, entries, "config/profiles.yaml")
}

// Absolute key_path values lose their dot segments before the target is compared; relative ones stay unresolved.
func TestExportKeyPathCleaningAndRelativeValues(t *testing.T) {
	t.Parallel()
	entries := exportedKeyTree(t, map[string]string{
		"config/notifications/http_default.yaml": httpPlugin("key_path: /etc/crowdsec/ssl/../ssl/a.pem\n"),
		"config/notifications/http_other.yaml":   strings.Replace(httpPlugin("key_path: ssl/relative.pem\n"), "http_default", "http_other", 1),
		"config/ssl/a.pem":                       "opaque\n",
		"config/ssl/relative.pem":                "plain text, no armor\n",
	})

	require.NotContains(t, entries, "config/ssl/a.pem", "dot segments in an absolute value are cleaned")
	require.Contains(t, entries, "config/ssl/relative.pem", "a relative value cannot be resolved statically")
}

func TestExportKeyPathThroughAMergeKeyIsHonoured(t *testing.T) {
	t.Parallel()
	content := "type: http\nname: http_default\nurl: https://hooks.example.invalid/n\n" +
		"base: &base\n  key_path: /etc/crowdsec/ssl/merged.pem\n<<: *base\n"
	entries := exportedKeyTree(t, map[string]string{
		"config/notifications/http_default.yaml": content,
		"config/ssl/merged.pem":                  "opaque\n",
	})

	require.NotContains(t, entries, "config/ssl/merged.pem")
}
