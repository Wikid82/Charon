package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleCommandExecutor_Execute(t *testing.T) {
	t.Parallel()
	e := &simpleCommandExecutor{}

	out, err := e.Execute(t.Context(), "sh", "-c", "echo out; echo err >&2")
	require.NoError(t, err)
	assert.Equal(t, "out\nerr\n", string(out))

	_, err = e.Execute(t.Context(), "sh", "-c", "exit 2")
	require.Error(t, err)
}
