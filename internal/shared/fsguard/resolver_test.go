package fsguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolverBlocksTraversalAndHiddenPaths(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootDir, "visible"), 0o755); err != nil {
		t.Fatalf("mkdir visible: %v", err)
	}
	if err := os.Mkdir(filepath.Join(rootDir, ".hidden"), 0o755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}

	resolver, err := New([]string{"repo=" + rootDir}, false)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	if _, err := resolver.Resolve("repo", "../escape"); err == nil {
		t.Fatalf("expected traversal to be blocked")
	}
	if _, err := resolver.Resolve("repo", ".hidden"); err == nil {
		t.Fatalf("expected hidden path to be blocked")
	}
	if _, err := resolver.Resolve("repo", "visible"); err != nil {
		t.Fatalf("expected visible path to resolve, got %v", err)
	}
}

func TestResolverListRoots(t *testing.T) {
	rootDir := t.TempDir()
	resolver, err := New([]string{"repo=" + rootDir}, false)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	roots := resolver.ListRoots()
	if len(roots) != 1 || roots[0].Name != "repo" {
		t.Fatalf("unexpected roots: %#v", roots)
	}
}
