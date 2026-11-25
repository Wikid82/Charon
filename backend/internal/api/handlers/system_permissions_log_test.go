package handlers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizedErrorText_NilErrorIsEmpty(t *testing.T) {
	require.Empty(t, sanitizedErrorText(nil))
}

func TestSanitizedErrorText_StripsLineBreaksAndControlChars(t *testing.T) {
	err := errors.New("failed\nINFO forged entry\r\n\x1b[31m end")
	got := sanitizedErrorText(err)
	require.NotContains(t, got, "\n")
	require.NotContains(t, got, "\r")
	require.NotContains(t, got, "\x1b")
	require.Contains(t, got, "forged entry")
}

func TestRepairFailure_ReturnsFixedResultForLoggedError(t *testing.T) {
	result := repairFailure("/data/x", "permissions_repair_failed", "could not change the mode", errors.New("line one\nline two"))
	require.Equal(t, "permissions_repair_failed", result.ErrorCode)
	require.Equal(t, "could not change the mode", result.Message)
}
