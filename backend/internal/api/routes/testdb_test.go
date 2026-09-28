package routes

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// isolatedMemoryDSN returns a private in-memory SQLite DSN unique to this
// test invocation.
// SQLite keys shared-cache in-memory databases by URI path, so trailing
// "&label" params do not isolate anything, and a fixed name (even t.Name())
// survives across -count=N runs while any pooled connection stays open.
func isolatedMemoryDSN(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())
}
