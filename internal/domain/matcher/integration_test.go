package matcher

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/lHumaNl/gomock/internal/configloader"
)

func TestSelectLoadedMappings(t *testing.T) {
	root := newLoadedMappingRoot(t)
	writeLoadedMapping(t, root, "mappings/general.yaml", loadedMappingYAML("general", 5, false))
	writeLoadedMapping(t, root, "mappings/specific.yaml", loadedMappingYAML("specific", 5, true))

	items, err := configloader.NewLoader(false).LoadRoot(root)
	if err != nil {
		t.Fatalf("load mappings: %v", err)
	}

	selection := Select(items, Request{Method: "GET", URI: "/users?active=true"})

	if !selection.Found() {
		t.Fatalf("expected selected mapping, got %#v", selection)
	}
	if selection.Mapping.ID != "specific" {
		t.Fatalf("expected specific mapping, got %q", selection.Mapping.ID)
	}
}

func loadedMappingYAML(id string, priority int, hasQueryMatcher bool) string {
	content := "id: " + id + "\npriority: " + strconv.Itoa(priority) + "\nrequest:\n  method: get\n  urlPath: /users\n"
	if hasQueryMatcher {
		content += "  queryParameters:\n    active:\n      equalTo: \"true\"\n"
	}
	return content + "response:\n  status: 200\n  body: ok\n"
}

func newLoadedMappingRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mappings"), 0o755); err != nil {
		t.Fatalf("mkdir mappings: %v", err)
	}
	return root
}

func writeLoadedMapping(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write mapping: %v", err)
	}
}
