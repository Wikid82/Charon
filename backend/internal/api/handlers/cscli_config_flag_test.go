package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil/cscliguard"
)

// TestCSCLIStartsUseConfigFlag keeps every cscli start in this package on crowdsec.RunCSCLI, which passes the
// engine configuration file as -c. A direct start with a literal "cscli" command would drop that flag.
func TestCSCLIStartsUseConfigFlag(t *testing.T) {
	t.Parallel()

	offenders, err := cscliguard.Offenders(".", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, offenders)
}
