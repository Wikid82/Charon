package crowdsec

import (
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitedBuffer_Boundaries(t *testing.T) {
	t.Parallel()

	t.Run("exactly at the cap is accepted", func(t *testing.T) {
		t.Parallel()
		b := &limitedBuffer{limit: 4}
		n, err := b.Write([]byte("abcd"))
		require.NoError(t, err)
		assert.Equal(t, 4, n)
		out, exceeded := b.snapshot()
		assert.Equal(t, "abcd", string(out))
		assert.False(t, exceeded)
	})

	t.Run("one byte past the cap is rejected after a short write", func(t *testing.T) {
		t.Parallel()
		fired := 0
		b := &limitedBuffer{limit: 4, onLimit: func() { fired++ }}
		n, err := b.Write([]byte("abcde"))
		require.ErrorIs(t, err, ErrOutputLimit)
		assert.Equal(t, 4, n)
		out, exceeded := b.snapshot()
		assert.Equal(t, "abcd", string(out))
		assert.True(t, exceeded)

		n, err = b.Write([]byte("x"))
		require.ErrorIs(t, err, ErrOutputLimit)
		assert.Zero(t, n)
		assert.Equal(t, 1, fired, "the limit callback fires once")
	})

	t.Run("a byte after a full buffer is rejected without data", func(t *testing.T) {
		t.Parallel()
		b := &limitedBuffer{limit: 2}
		_, err := b.Write([]byte("ab"))
		require.NoError(t, err)
		n, err := b.Write([]byte("c"))
		require.ErrorIs(t, err, ErrOutputLimit)
		assert.Zero(t, n)
		out, _ := b.snapshot()
		assert.Equal(t, "ab", string(out))
	})

	t.Run("short writes accumulate", func(t *testing.T) {
		t.Parallel()
		b := &limitedBuffer{limit: 6}
		for _, s := range []string{"a", "bc", "def"} {
			n, err := b.Write([]byte(s))
			require.NoError(t, err)
			assert.Equal(t, len(s), n)
		}
		out, exceeded := b.snapshot()
		assert.Equal(t, "abcdef", string(out))
		assert.False(t, exceeded)
	})
}

func TestLimitedBuffer_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	const writers, perWriter = 8, 100
	b := &limitedBuffer{limit: writers * perWriter / 2}
	var wg sync.WaitGroup
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWriter {
				_, _ = b.Write([]byte("x"))
			}
		}()
	}
	wg.Wait()
	out, exceeded := b.snapshot()
	assert.Len(t, out, b.limit)
	assert.True(t, exceeded)
}

func TestCombinedOutputLimited(t *testing.T) {
	t.Parallel()

	t.Run("keeps both streams in order", func(t *testing.T) {
		t.Parallel()
		out, err := CombinedOutputLimited(exec.CommandContext(t.Context(), "sh", "-c", "echo out; echo err >&2"))
		require.NoError(t, err)
		assert.Equal(t, "out\nerr\n", string(out))
	})

	t.Run("reports the command failure with its output", func(t *testing.T) {
		t.Parallel()
		out, err := CombinedOutputLimited(exec.CommandContext(t.Context(), "sh", "-c", "echo bad >&2; exit 3"))
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, "bad\n", string(out))
	})

	t.Run("reports a missing binary", func(t *testing.T) {
		t.Parallel()
		_, err := CombinedOutputLimited(exec.CommandContext(t.Context(), "cscli-does-not-exist"))
		require.Error(t, err)
	})

	t.Run("kills a command and its orphaned child past the limit", func(t *testing.T) {
		t.Parallel()
		start := time.Now()
		// The background sleep inherits the pipes and outlives the shell.
		out, err := CombinedOutputLimited(exec.CommandContext(t.Context(), "sh", "-c", "sleep 30 & head -c 40000000 /dev/zero; wait"))
		require.ErrorIs(t, err, ErrOutputLimit)
		assert.Less(t, time.Since(start), 4*time.Second)
		assert.Len(t, out, maxCapturedOutput)
	})
}

func TestLimitedBuffer_SpansChunks(t *testing.T) {
	t.Parallel()
	b := &limitedBuffer{limit: 2*outputChunkSize + 10}
	want := make([]byte, 0, b.limit)
	for i := 0; len(want) < b.limit; i++ {
		p := []byte(strings.Repeat(string(rune('a'+i%26)), 700_000))
		p = p[:min(len(p), b.limit-len(want))]
		n, err := b.Write(p)
		require.NoError(t, err)
		require.Equal(t, len(p), n)
		want = append(want, p...)
	}
	out, exceeded := b.snapshot()
	assert.False(t, exceeded)
	assert.Equal(t, want, out)
}
