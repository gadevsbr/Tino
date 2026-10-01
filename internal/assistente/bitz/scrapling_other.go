//go:build !windows

package bitz

import "os/exec"

func hideScraplingWindow(cmd *exec.Cmd) {}
