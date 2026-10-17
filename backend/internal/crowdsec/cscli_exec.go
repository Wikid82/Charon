package crowdsec

import (
	"errors"
	"os/exec"
	"sync"
	"time"
)

// maxCapturedOutput is the most output read from one cscli command. It sits just above maxCSCLIOutput so a
// document the parsers accept is never cut short, while a runaway command is stopped instead of filling memory.
const maxCapturedOutput = maxCSCLIOutput + 1<<20

// outputWaitDelay bounds how long a finished or killed command may keep its output pipes open, so a child that
// outlives the command cannot hold the call.
const outputWaitDelay = 2 * time.Second

// outputChunkSize is the size of one block of captured output.
const outputChunkSize = 1 << 20

// ErrOutputLimit is returned when a command prints more output than the limit allows. It never carries output.
var ErrOutputLimit = errors.New("command output exceeded the size limit")

// limitedBuffer collects written bytes up to limit and rejects the rest. A single buffer may be shared by the
// stdout and stderr of one command, so writes are serialized and their interleaving is preserved.
type limitedBuffer struct {
	mu       sync.Mutex
	chunks   [][]byte // fixed-size blocks, so growing never copies what is already stored
	size     int
	limit    int
	exceeded bool
	onLimit  func()
}

// Write stores as much of p as fits. A write that does not fit entirely stores the part that does, fires
// onLimit once and returns ErrOutputLimit, which stops the copying of the stream.
func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	if b.exceeded {
		b.mu.Unlock()
		return 0, ErrOutputLimit
	}
	room := b.limit - b.size
	if len(p) <= room {
		b.store(p)
		b.mu.Unlock()
		return len(p), nil
	}
	b.store(p[:room])
	b.exceeded = true
	onLimit := b.onLimit
	b.mu.Unlock()
	if onLimit != nil {
		onLimit()
	}
	return room, ErrOutputLimit
}

// store appends p to the chunks; the caller holds mu.
func (b *limitedBuffer) store(p []byte) {
	b.size += len(p)
	for len(p) > 0 {
		if n := len(b.chunks); n == 0 || len(b.chunks[n-1]) == cap(b.chunks[n-1]) {
			b.chunks = append(b.chunks, make([]byte, 0, outputChunkSize))
		}
		last := &b.chunks[len(b.chunks)-1]
		fit := min(len(p), cap(*last)-len(*last))
		*last = append(*last, p[:fit]...)
		p = p[fit:]
	}
}

// snapshot returns the stored bytes joined into one slice and whether the limit was exceeded.
func (b *limitedBuffer) snapshot() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, 0, b.size)
	for _, c := range b.chunks {
		out = append(out, c...)
	}
	return out, b.exceeded
}

// CombinedOutputLimited runs cmd like cmd.CombinedOutput but reads at most maxCapturedOutput bytes. Past the
// limit the command and its process group are killed and ErrOutputLimit is returned with the bytes kept.
// cmd must have been created with exec.CommandContext.
func CombinedOutputLimited(cmd *exec.Cmd) ([]byte, error) {
	out := &limitedBuffer{limit: maxCapturedOutput, onLimit: func() { killProcessTree(cmd) }}
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = outputWaitDelay
	cmd.Cancel = func() error { killProcessTree(cmd); return nil }
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err //nolint:wrapcheck // same error cmd.Run reports; callers match on *exec.Error
	}
	waitErr := cmd.Wait()
	data, exceeded := out.snapshot()
	if exceeded {
		return data, ErrOutputLimit
	}
	return data, waitErr
}
