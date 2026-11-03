package bouncerkey

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHeldKeyInUseMessageSaved pins the wording used when a held key was written to the key file.
func TestHeldKeyInUseMessageSaved(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"CrowdSec bouncer: the key held in memory was saved to the key file and is in use",
		HeldKeyInUseMessage(true))
}

// TestHeldKeyInUseMessageNotSaved pins the wording used when a held key could not be written to the key file.
func TestHeldKeyInUseMessageNotSaved(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"CrowdSec bouncer: the key held in memory is still in use; the key file could not be written",
		HeldKeyInUseMessage(false))
}
