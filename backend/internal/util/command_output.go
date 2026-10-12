package util

import "fmt"

// DescribeCommandOutput summarises command output for diagnostics without
// including its content: only the length is reported.
func DescribeCommandOutput(out []byte) string {
	if len(out) == 0 {
		return "no output"
	}
	return fmt.Sprintf("%d bytes of output omitted", len(out))
}
