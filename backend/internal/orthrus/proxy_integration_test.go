//go:build integration

package orthrus

import "testing"

func TestDockerProxy_Integration(t *testing.T) {
	t.Skip("requires running Orthrus agent with /var/run/docker.sock")
}
