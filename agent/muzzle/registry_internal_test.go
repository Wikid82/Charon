package muzzle

// Mirrors TestIsAllDigits in backend/internal/orthrus/muzzle_registry_test.go.

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAllDigits(t *testing.T) {
	assert.False(t, isAllDigits(""))
	assert.True(t, isAllDigits("127"))
	assert.False(t, isAllDigits("1a"))
}

// failingDeadlineReader is a stream whose read deadline cannot be set.
type failingDeadlineReader struct {
	io.Reader
}

func (failingDeadlineReader) SetReadDeadline(time.Time) error {
	return errors.New("deadline unsupported")
}

// TestFilter_ServeProxy_DeadlineErrorFailsClosed proves the proxy refuses a
// stream when the header deadline cannot be set, instead of reading without it.
func TestFilter_ServeProxy_DeadlineErrorFailsClosed(t *testing.T) {
	f := New(false)
	r := failingDeadlineReader{Reader: strings.NewReader("GET /_ping HTTP/1.1\r\nHost: docker\r\n\r\n")}

	err := f.ServeProxy("/nonexistent.sock", r, io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set header deadline")
}
