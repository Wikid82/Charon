package crowdsec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalCSCLIOutput(t *testing.T) {
	t.Parallel()
	var got []int
	require.NoError(t, UnmarshalCSCLIOutput([]byte("Loaded: 1 parsers\n[1,2]\n"), &got))
	assert.Equal(t, []int{1, 2}, got)
	assert.Error(t, UnmarshalCSCLIOutput([]byte("no json"), &got))
}
