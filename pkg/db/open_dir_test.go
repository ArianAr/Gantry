package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesParentDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "dir", "test.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file missing: %v", err)
	}
}
