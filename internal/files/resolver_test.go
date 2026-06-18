package files

import (
	"strings"
	"testing"
)

func TestResolverRejectsPathTraversal(t *testing.T) {
	resolver := NewResolver(t.TempDir())

	_, err := resolver.Resolve("../secret.json")

	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestResolverResolvesRelativeFileUnderFilesRoot(t *testing.T) {
	resolver := NewResolver("/mock-root")

	path, err := resolver.Resolve("users/list.json")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.HasSuffix(path, "/mock-root/__files/users/list.json") {
		t.Fatalf("unexpected resolved path %q", path)
	}
}
