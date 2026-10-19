//go:build !unix

package crowdsec

import "os/exec"

func setProcessGroup(*exec.Cmd) {}

// killProcessTree kills the command's process.
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
