package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPDFFontFindsBundledTTF(t *testing.T) {
	root := t.TempDir()
	fontDir := filepath.Join(root, "iransans")
	if err := os.MkdirAll(fontDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fontDir, "IRANSansX-Regular.ttf"), []byte("test font"), 0644); err != nil {
		t.Fatal(err)
	}
	dir, name, err := findPDFFont(root)
	if err != nil {
		t.Fatal(err)
	}
	if dir != fontDir || name != "IRANSansX-Regular.ttf" {
		t.Fatalf("unexpected font location: %q %q", dir, name)
	}
}
