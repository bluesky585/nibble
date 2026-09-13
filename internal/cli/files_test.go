package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListDirFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.md"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "c.go"), []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}

	got, err := listDirFiles(root, ".txt,.md")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.txt", filepath.Join("sub", "b.md")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseExts(t *testing.T) {
	t.Parallel()

	got := parseExts("md, .TXT,")
	want := []string{".md", ".txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
