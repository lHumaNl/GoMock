package configloader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func TestLoadRootLoadsYAMLAndJSONMappings(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "__files/users.json", `{"users":[]}`)
	writeFile(t, root, "mappings/get-users.yaml", validYAMLMapping())
	writeFile(t, root, "mappings/create-user.json", validJSONMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(items))
	}
	assertLoadedYAMLMapping(t, items[1])
}

func TestLoadRootLoadsJSON5CompatibleMapping(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/json5.json", json5CompatibleMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(items))
	}
	assertLoadedJSON5Mapping(t, items[0])
}

func TestLoadRootRejectsMissingRequest(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "response:\n  status: 200\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request", "is required")
}

func TestLoadRootRejectsInvalidURLMatcherShape(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "request:\n  url: /a\n  urlPath: /a\nresponse:\n  status: 200\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request.url", "exactly one")
}

func TestLoadRootRejectsMissingResponseStatus(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "request:\n  urlPath: /a\nresponse:\n  body: ok\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "response.status", "is required")
}

func TestLoadRootRejectsMutuallyExclusiveBodyFields(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "request:\n  urlPath: /a\nresponse:\n  status: 200\n  body: ok\n  bodyFileName: ok.txt\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "response.body", "mutually exclusive")
}

func TestLoadRootRejectsUnsupportedOperators(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "request:\n  urlPath: /a\n  headers:\n    X-ID:\n      before: x\nresponse:\n  status: 200\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request.headers.X-ID", "unsupported operator")
}

func TestLoadRootRejectsInvalidRegex(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "request:\n  urlPattern: '['\nresponse:\n  status: 200\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request.urlPattern", "invalid regex")
}

func TestLoadRootRejectsInvalidRandomDelay(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "request:\n  urlPath: /a\nresponse:\n  status: 200\n  delay:\n    type: random\n    min: 2s\n    max: 1s\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "response.delay.min", "less than or equal")
}

func TestLoadRootRejectsTraversalBodyFile(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "request:\n  urlPath: /a\nresponse:\n  status: 200\n  bodyFileName: ../secret.txt\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "response.bodyFileName", "path traversal")
}

func TestLoadRootRejectsUnsupportedResponseMode(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "request:\n  urlPath: /a\nresponses:\n  mode: rotate\n  variants:\n    - status: 200\n")

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "responses.mode", "sequential")
}

func TestLoadRootBuildsWeightedVariants(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/variants.yaml", "request:\n  urlPath: /a\nresponses:\n  mode: weighted\n  variants:\n    - status: 200\n      weight: 3\n    - name: error\n      status: 500\n      weight: 1\n")

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if items[0].Responses.Variants[0].Name != "variant-0" {
		t.Fatalf("expected generated variant name, got %q", items[0].Responses.Variants[0].Name)
	}
}

func TestLoadRootUsesWireMockLikeDefaultPriority(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "__files/users.json", `{"users":[]}`)
	writeFile(t, root, "mappings/default.yaml", validYAMLMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if items[0].Priority != mapping.DefaultPriority {
		t.Fatalf("expected default priority %d, got %d", mapping.DefaultPriority, items[0].Priority)
	}
}

func TestLoadRootPreservesExplicitZeroPriority(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "__files/users.json", `{"users":[]}`)
	writeFile(t, root, "mappings/explicit.yaml", "priority: 0\n"+validYAMLMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if items[0].Priority != 0 {
		t.Fatalf("expected explicit priority 0, got %d", items[0].Priority)
	}
}

func TestLoadRootStrictModeRejectsUnknownYAMLFields(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", "unknown: true\nrequest:\n  urlPath: /a\nresponse:\n  status: 200\n")

	_, err := NewLoader(true).LoadRoot(root)

	assertErrorContains(t, err, "bad.yaml", "field", "unknown")
}

func TestLoadRootStrictModeRejectsUnknownJSON5Fields(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.json", unknownJSON5FieldMapping())

	_, err := NewLoader(true).LoadRoot(root)

	assertErrorContains(t, err, "bad.json", "unknown", "extra")
}

func assertLoadedYAMLMapping(t *testing.T, item mapping.Mapping) {
	t.Helper()
	if item.ID != "get-users" || item.Request.Method != "GET" {
		t.Fatalf("unexpected mapping identity: %#v", item)
	}
	if item.Response.Delay.Value != 500*time.Millisecond {
		t.Fatalf("unexpected delay: %#v", item.Response.Delay)
	}
	if string(item.Response.BodyFileContent) != `{"users":[]}` {
		t.Fatalf("unexpected body file content %q", item.Response.BodyFileContent)
	}
}

func assertLoadedJSON5Mapping(t *testing.T, item mapping.Mapping) {
	t.Helper()
	if item.ID != "json5-user" || item.Request.Method != "GET" {
		t.Fatalf("unexpected mapping identity: %#v", item)
	}
	if item.Request.Headers["X-Client"].Value != "web" {
		t.Fatalf("unexpected header matcher: %#v", item.Request.Headers)
	}
	if item.Response.Body != `{"ok":true}` {
		t.Fatalf("unexpected response body %q", item.Response.Body)
	}
}

func validYAMLMapping() string {
	return "name: Get users\nrequest:\n  method: get\n  urlPath: /api/users\nresponse:\n  status: 200\n  bodyFileName: users.json\n  delay:\n    type: fixed\n    value: 500ms\n"
}

func validJSONMapping() string {
	return `{"id":"create-user","request":{"method":"post","url":"/api/users"},"response":{"status":201,"body":"ok"}}`
}

func json5CompatibleMapping() string {
	return `// JSON5-style comment for WireMock migration.
{
  id: 'json5-user',
  request: {
    method: 'get',
    urlPath: '/api/json5',
    headers: {'X-Client': {contains: 'web'}},
  },
  /* block comments and trailing commas are accepted in .json mappings */
  response: {
    status: 200,
    body: '{"ok":true}',
  },
}`
}

func unknownJSON5FieldMapping() string {
	return `{
  request: {urlPath: '/a', extra: true},
  response: {status: 200},
}`
}

func loadRootError(root string) error {
	_, err := NewLoader(false).LoadRoot(root)
	return err
}

func assertErrorContains(t *testing.T, err error, parts ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error")
	}
	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("expected error %q to contain %q", err.Error(), part)
		}
	}
}

func newMockRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "mappings"))
	mustMkdir(t, filepath.Join(root, "__files"))
	return root
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
