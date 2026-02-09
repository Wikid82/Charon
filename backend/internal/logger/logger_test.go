package logger

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBroadcastHook(t *testing.T) {
	hook := NewBroadcastHook()
	assert.NotNil(t, hook)
	assert.NotNil(t, hook.listeners)
	assert.Equal(t, 0, len(hook.listeners))
}

func TestBroadcastHook_Levels(t *testing.T) {
	hook := NewBroadcastHook()
	levels := hook.Levels()
	assert.Equal(t, logrus.AllLevels, levels)
}

func TestBroadcastHook_Subscribe(t *testing.T) {
	hook := NewBroadcastHook()
	ch := hook.Subscribe("test-id")
	assert.NotNil(t, ch)
	assert.Equal(t, 1, hook.ActiveListeners())
}

func TestBroadcastHook_Unsubscribe(t *testing.T) {
	hook := NewBroadcastHook()
	ch := hook.Subscribe("test-id")
	assert.NotNil(t, ch)
	assert.Equal(t, 1, hook.ActiveListeners())

	hook.Unsubscribe("test-id")
	assert.Equal(t, 0, hook.ActiveListeners())

	// Test unsubscribe non-existent ID (should not panic)
	hook.Unsubscribe("non-existent")
}

func TestInit(t *testing.T) {
	var buf bytes.Buffer

	// Test with debug mode
	Init(true, &buf)
	assert.NotNil(t, _log)
	assert.Equal(t, logrus.DebugLevel, _log.Level)

	// Test without debug mode
	buf.Reset()
	Init(false, &buf)
	assert.Equal(t, logrus.InfoLevel, _log.Level)

	// Test with nil output (should use stdout)
	Init(false, nil)
	assert.NotNil(t, _log.Out)
}

func TestLog(t *testing.T) {
	Init(false, nil)
	entry := Log()
	assert.NotNil(t, entry)
	assert.Equal(t, _log, entry.Logger)
}

func TestWithFields(t *testing.T) {
	Init(false, nil)
	fields := logrus.Fields{
		"key1": "value1",
		"key2": "value2",
	}
	entry := WithFields(fields)
	assert.NotNil(t, entry)
	assert.Equal(t, "value1", entry.Data["key1"])
	assert.Equal(t, "value2", entry.Data["key2"])
}

func TestBroadcastHook_Fire(t *testing.T) {
	hook := NewBroadcastHook()
	ch := hook.Subscribe("test-id")

	entry := &logrus.Entry{
		Logger:  logrus.New(),
		Message: "test message",
		Level:   logrus.InfoLevel,
	}

	err := hook.Fire(entry)
	require.NoError(t, err)

	// Verify we can receive the entry
	select {
	case receivedEntry := <-ch:
		assert.NotNil(t, receivedEntry)
		assert.Equal(t, "test message", receivedEntry.Message)
	default:
		t.Fatal("Expected to receive log entry")
	}
}

func TestGetBroadcastHook(t *testing.T) {
	// Reset global hook
	_broadcastHook = nil

	hook := GetBroadcastHook()
	assert.NotNil(t, hook)

	// Call again to test cached hook
	hook2 := GetBroadcastHook()
	assert.Equal(t, hook, hook2)
}
