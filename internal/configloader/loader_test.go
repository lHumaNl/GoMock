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

func TestLoadRootLoadsWireMockMappingsArray(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/CALLBACK_ow.json", wireMockMappingsArray())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(items))
	}
	if items[0].ID != "CALLBACK_ow-0-callback-created" {
		t.Fatalf("unexpected generated first mapping ID %q", items[0].ID)
	}
	if items[1].ID != "CALLBACK_ow-1" {
		t.Fatalf("unexpected generated second mapping ID %q", items[1].ID)
	}
	if items[0].Name != "Callback Created" || items[0].Request.URLValue != "/callback" {
		t.Fatalf("unexpected first mapping: %#v", items[0])
	}
	if items[1].Request.URLValue != "/callback/2" || items[1].Response.Status != 201 {
		t.Fatalf("unexpected second mapping: %#v", items[1])
	}
}

func TestLoadRootMappingsArrayReportsItemIndexInErrors(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.json", `{
	  "mappings": [
	    {"request": {"urlPath": "/ok"}, "response": {"status": 200}},
	    {"response": {"status": 200}}
	  ]
	}`)

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.json:mappings[1]", "request", "is required")
}

func TestLoadRootStrictModeRejectsUnsupportedWireMockMappingFields(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/strict.json", `{
	  "mappings": [
	    {
	      "request": {"urlPath": "/callback"},
	      "response": {"status": 200},
	      "serveEventListeners": [{"name": "webhook"}]
	    }
	  ]
	}`)

	_, err := NewLoader(true).LoadRoot(root)

	assertErrorContains(t, err, "strict.json", "unknown", "serveEventListeners")
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

func TestLoadRootAcceptsMatchesXPathBodyPattern(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/xml.yaml", xmlXPathMapping("//*[local-name()='cus']"))

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	matcher := items[0].Request.BodyPatterns[0]
	if matcher.Operator != mapping.OperatorMatchesXPath {
		t.Fatalf("unexpected body matcher: %#v", matcher)
	}
}

func TestLoadRootRejectsInvalidXPath(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", xmlXPathMapping("//*["))

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request.bodyPatterns[0].matchesXPath", "invalid XPath")
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

func TestLoadRootPreservesResponseTransformers(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/transformer.json", responseTransformerMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if items[0].Response.Status != 200 {
		t.Fatalf("unexpected response: %#v", items[0].Response)
	}
	if !items[0].Response.HasTransformer(mapping.TransformerResponseTemplate) {
		t.Fatalf("expected response-template transformer: %#v", items[0].Response.Transformers)
	}
}

func TestLoadRootStrictModeAcceptsResponseTransformers(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/transformer.json", responseTransformerMapping())

	items, err := NewLoader(true).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !items[0].Response.HasTransformer(mapping.TransformerResponseTemplate) {
		t.Fatalf("expected response-template transformer: %#v", items[0].Response.Transformers)
	}
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

func wireMockMappingsArray() string {
	return `// JSON5-style comment for WireMock migration.
{
  mappings: [
    {
      name: 'Callback Created',
      request: {method: 'post', urlPath: '/callback'},
      response: {status: 200, body: 'ok'},
      serveEventListeners: [{name: 'webhook', parameters: {}}],
    },
    {
      request: {method: 'post', urlPath: '/callback/2'},
      response: {status: 201, body: 'created'},
    },
  ],
}`
}

func unknownJSON5FieldMapping() string {
	return `{
  request: {urlPath: '/a', extra: true},
  response: {status: 200},
}`
}

func xmlXPathMapping(expression string) string {
	return "request:\n  urlPath: /soap\n  bodyPatterns:\n    - matchesXPath: \"" + expression + "\"\nresponse:\n  status: 200\n"
}

func responseTransformerMapping() string {
	return `{
  request: {urlPath: '/templated'},
  response: {status: 200, body: 'ok', transformers: ['response-template']},
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
