package crowdsec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRawIndexReadsVersionDigest(t *testing.T) {
	good := sha256Hex("payload")
	cases := []struct {
		name     string
		versions string
		want     string
	}{
		{"present digest", versionsWithDigest(good), good},
		{"upper-case digest with whitespace is normalised", versionsWithDigest(" " + good[:10] + "ABCDEF" + good[16:] + " "), good[:10] + "abcdef" + good[16:]},
		{"absent versions key leaves digest empty", "", ""},
		{"version missing from the record map leaves digest empty", `{"0.9":{"digest":"` + good + `"}}`, ""},
		{"malformed digest is kept for the verifier to refuse", versionsWithDigest("not-a-digest"), "not-a-digest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx, err := parseRawIndex([]byte(rawDigestIndex(tc.versions)), "https://hub.crowdsec.net")
			require.NoError(t, err)
			require.Len(t, idx.Items, 1)
			require.Equal(t, tc.want, idx.Items[0].Digest)
		})
	}
}

func TestParseRawIndexDigestNotInJSON(t *testing.T) {
	idx, err := parseRawIndex([]byte(rawDigestIndex(versionsWithDigest(sha256Hex("payload")))), "https://hub.crowdsec.net")
	require.NoError(t, err)
	require.NotEmpty(t, idx.Items[0].Digest)
	raw, err := json.Marshal(idx.Items[0])
	require.NoError(t, err)
	require.NotContains(t, string(raw), "digest")
}
