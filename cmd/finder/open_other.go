//go:build !windows

package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

func opener() string {
	if runtime.GOOS == "darwin" {
		return "open"
	}
	return "xdg-open"
}

func openFile(p string) error   { return exec.Command(opener(), p).Start() }
func revealFile(p string) error { return exec.Command(opener(), filepath.Dir(p)).Start() }
