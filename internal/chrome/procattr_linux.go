package chrome

import (
	"os/exec"
	"syscall"
)

// endWithThisProcess has Linux end the browser when the test process that
// started it dies, whatever the way.
func endWithThisProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
