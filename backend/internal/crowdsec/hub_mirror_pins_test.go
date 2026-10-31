package crowdsec

import "testing"

// The tests below pin behaviour that later commits of the hub outage fallback implement. They stay skipped
// here, and each commit removes the skip for its own pins.

func TestHubPinPullTriesMirrorWhenFirstAddressHangs(t *testing.T) {
	t.Skip("enabled in commit 3")
}

func TestHubPinFetchIndexWithoutDeadlineBoundedWhenBothAddressesHang(t *testing.T) {
	t.Skip("enabled in commit 3")
}

func TestHubPinCSCLIRetriesOnMirrorAfterNetworkFailure(t *testing.T) {
	t.Skip("enabled in commit 4")
}

func TestHubPinCSCLIDoesNotRetryOnNonNetworkFailure(t *testing.T) {
	t.Skip("enabled in commit 4")
}

func TestHubPinPresetListReportsHubOffline(t *testing.T) {
	t.Skip("enabled in commit 6")
}
