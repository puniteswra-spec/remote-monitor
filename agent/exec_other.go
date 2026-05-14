//go:build !windows

package main

import "os/exec"

func hideCmd(cmd *exec.Cmd) {
	// No-op on non-Windows
}
