package configloader

import (
	"testing"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func TestLoadRootBuildsWireMockCompatibilityFields(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/compat.json", wireMockCompatMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertCompatMapping(t, items[0])
}

func TestLoadRootAcceptsBasicAuthAlias(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/basic-auth.json", `{
	  request: {urlPath: '/secure', basicAuth: {username: 'api', password: 'secret'}},
	  response: {status: 200}
	}`)

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if items[0].Request.BasicAuth.Username != "api" || items[0].Request.BasicAuth.Password != "secret" {
		t.Fatalf("unexpected basicAuth: %#v", items[0].Request.BasicAuth)
	}
}

func TestLoadRootRejectsInvalidBase64Body(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.json", `{
	  request: {urlPath: '/binary'},
	  response: {status: 200, base64Body: 'not-base64!'}
	}`)

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.json", "response.base64Body", "invalid base64")
}

func TestLoadRootRejectsInvalidURLPathPattern(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.json", `{
	  request: {urlPathPattern: '['},
	  response: {status: 200}
	}`)

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.json", "request.urlPathPattern", "invalid regex")
}

func TestLoadRootRejectsNegativeFixedDelayMilliseconds(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.json", `{
	  request: {urlPath: '/slow'},
	  response: {status: 200, fixedDelayMilliseconds: -1}
	}`)

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.json", "response.fixedDelayMilliseconds", "non-negative")
}

func TestLoadRootRejectsNonBooleanCaseInsensitive(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.json", `{
	  request: {urlPath: '/search', headers: {'X-ID': {equalTo: 'a', caseInsensitive: 'yes'}}},
	  response: {status: 200}
	}`)

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.json", "request.headers.X-ID.caseInsensitive", "boolean")
}

func TestLoadRootRejectsUnsupportedCaseInsensitiveOperator(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.json", `{
	  request: {urlPath: '/search', headers: {'X-ID': {matches: 'abc.*', caseInsensitive: true}}},
	  response: {status: 200}
	}`)

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.json", "request.headers.X-ID.caseInsensitive", "only supported")
}

func TestLoadRootRejectsPathParametersWithoutURLPathTemplate(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.json", `{
	  request: {urlPath: '/contacts/c123', pathParameters: {contactId: {equalTo: 'c123'}}},
	  response: {status: 200}
	}`)

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.json", "request.pathParameters", "requires request.urlPathTemplate")
}

func assertCompatMapping(t *testing.T, item mapping.Mapping) {
	t.Helper()
	if item.Request.URLKind != mapping.URLMatchKindURLPathPattern || item.Request.URLValue != "^/binary/[0-9]+$" {
		t.Fatalf("unexpected URL matcher: %#v", item.Request)
	}
	if !item.Request.Headers["X-Trace"].CaseInsensitive || !item.Request.Cookies["session"].CaseInsensitive {
		t.Fatalf("expected case-insensitive matchers: %#v", item.Request)
	}
	if item.Request.BasicAuth.Username != "api" || item.Request.BasicAuth.Password != "secret" {
		t.Fatalf("unexpected basic auth: %#v", item.Request.BasicAuth)
	}
	if item.Response.Delay.Value != 25*time.Millisecond || string(item.Response.BodyBytes) != "hello" {
		t.Fatalf("unexpected response: %#v", item.Response)
	}
}

func wireMockCompatMapping() string {
	return `{
	  request: {
	    method: 'get',
	    urlPathPattern: '^/binary/[0-9]+$',
	    headers: {'X-Trace': {equalTo: 'ABC', caseInsensitive: true}},
	    cookies: {session: {contains: 'TOKEN', caseInsensitive: true}},
	    basicAuthCredentials: {username: 'api', password: 'secret'},
	  },
	  response: {status: 200, base64Body: 'aGVsbG8=', fixedDelayMilliseconds: 25},
	}`
}
