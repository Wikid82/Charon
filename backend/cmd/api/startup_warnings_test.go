package main

import (
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogStartupWarnings(t *testing.T) {
	base, hook := logtest.NewNullLogger()
	logStartupWarnings(logrus.NewEntry(base), []string{"first problem", "second problem"})

	entries := hook.AllEntries()
	require.Len(t, entries, 2)
	for i, want := range []string{"first problem", "second problem"} {
		assert.Equal(t, logrus.WarnLevel, entries[i].Level)
		assert.Equal(t, want, entries[i].Message)
		assert.Equal(t, "config", entries[i].Data["component"])
	}

	hook.Reset()
	logStartupWarnings(logrus.NewEntry(base), nil)
	assert.Empty(t, hook.AllEntries())
}
