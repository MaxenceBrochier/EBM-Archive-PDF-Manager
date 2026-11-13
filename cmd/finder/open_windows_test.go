//go:build windows

package main

import (
	"path/filepath"
	"testing"
)

func TestOpenFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.pdf")
	if err := openFile(path); err == nil {
		t.Fatal("expected an error for a missing PDF")
	}
}
