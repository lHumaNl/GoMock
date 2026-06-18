package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Resolver struct {
	root string
}

func NewResolver(root string) *Resolver {
	return &Resolver{root: filepath.Join(root, "__files")}
}

func (r *Resolver) ReadBodyFile(name string) ([]byte, error) {
	path, err := r.Resolve(name)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read body file: %w", err)
	}

	return content, nil
}

func (r *Resolver) Resolve(name string) (string, error) {
	if err := validateRelativeName(name); err != nil {
		return "", err
	}

	path := filepath.Join(r.root, filepath.Clean(name))
	if !isWithinRoot(r.root, path) {
		return "", fmt.Errorf("path escapes __files root")
	}

	return path, nil
}

func validateRelativeName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("absolute paths are not allowed")
	}
	return rejectTraversal(name)
}

func rejectTraversal(name string) error {
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return fmt.Errorf("path traversal is not allowed")
		}
	}
	return nil
}

func isWithinRoot(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, "../")
}
