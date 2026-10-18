//go:build !linux

package chrome

import "os/exec"

// endWithThisProcess does nothing where the system cannot end a process with
// its parent; there, the next Start cleans up what a dead run left behind.
func endWithThisProcess(*exec.Cmd) {}
