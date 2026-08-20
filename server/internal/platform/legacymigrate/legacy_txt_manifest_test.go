package legacymigrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyTXTManifestRestrictsFilesToRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "book.txt"), []byte("chapter"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"root":".","files":{"42":"book.txt"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadLegacyTXTManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	path, err := manifest.Resolve(42)
	wantPath, resolveErr := filepath.EvalSymlinks(filepath.Join(root, "book.txt"))
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	if err != nil || path != wantPath {
		t.Fatalf("path=%q err=%v", path, err)
	}
	if _, err := manifest.Resolve(43); !os.IsNotExist(err) {
		t.Fatalf("missing task error=%v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(`{"root":".","files":{"44":"escape.txt"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	escape, err := LoadLegacyTXTManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := escape.Resolve(44); err == nil {
		t.Fatal("symlink outside root was accepted")
	}
}

func TestMapLegacyTXTStatus(t *testing.T) {
	status, interrupted, ok := mapLegacyTXTStatus("running")
	if !ok || status != "failed" || !interrupted {
		t.Fatalf("running=%q,%v,%v", status, interrupted, ok)
	}
	status, interrupted, ok = mapLegacyTXTStatus("success")
	if !ok || status != "succeeded" || interrupted {
		t.Fatalf("success=%q,%v,%v", status, interrupted, ok)
	}
	if _, _, ok := mapLegacyTXTStatus("unknown"); ok {
		t.Fatal("unknown status should be rejected")
	}
}
