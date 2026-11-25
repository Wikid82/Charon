package routes

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// TestLogRotationServiceFailure_RegistryFailureLogsError checks that a key
// registry or database failure is logged as an error with its own text.
func TestLogRotationServiceFailure_RegistryFailureLogsError(t *testing.T) {
	hook := logtest.NewLocal(logger.Log().Logger)
	t.Cleanup(hook.Reset)

	logRotationServiceFailure(fmt.Errorf("%w: register encryption keys: boom", crypto.ErrKeyRegistryUnavailable), true)

	require.Len(t, hook.AllEntries(), 1)
	entry := hook.LastEntry()
	assert.Equal(t, logrus.ErrorLevel, entry.Level)
	assert.Contains(t, entry.Message, "registry")
}

// TestLogRotationServiceFailure_KeyLoadFailureWarnsWhenConfigured checks that a key
// set that cannot be loaded keeps its warning, and only when a key was configured.
func TestLogRotationServiceFailure_KeyLoadFailureWarnsWhenConfigured(t *testing.T) {
	hook := logtest.NewLocal(logger.Log().Logger)
	t.Cleanup(hook.Reset)

	logRotationServiceFailure(errors.New("current is required"), true)
	require.Len(t, hook.AllEntries(), 1)
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	assert.Contains(t, hook.LastEntry().Message, "could not be loaded")

	hook.Reset()
	logRotationServiceFailure(errors.New("current is required"), false)
	assert.Empty(t, hook.AllEntries(), "nothing is logged when no key was configured")
}
