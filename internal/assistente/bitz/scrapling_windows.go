package bitz

import (
	"os/exec"
	"syscall"
)

func hideScraplingWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
