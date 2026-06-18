package configloader

import (
	"testing"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func TestLoadRootBuildsHasExactlyNamedMatchers(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/exact.json", hasExactlyMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertHasExactlyMatcher(t, items[0].Request.QueryParameters["syscode"], "UFONA")
	assertHasExactlyMatcher(t, items[0].Request.Headers["X-System"], "UFONA")
}

func TestLoadRootRejectsUnsupportedHasExactlyNestedOperator(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", unsupportedNestedHasExactlyMapping())

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request.queryParameters.syscode.hasExactly[0]", "unsupported operator before")
}

func TestLoadRootBuildsIncludesNamedMatchers(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/includes.json", includesMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertIncludesMatcher(t, items[0].Request.QueryParameters["syscode"], "UFONA")
	assertIncludesMatcher(t, items[0].Request.Headers["X-System"], "UFONA")
	assertIncludesMatcher(t, items[0].Request.Cookies["session"], "UFONA")
}

func TestLoadRootKeepsNestedCaseInsensitiveOnIncludes(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/includes.json", nestedCaseInsensitiveIncludesMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	nested := items[0].Request.QueryParameters["syscode"].ValueMatchers[0]
	if !nested.CaseInsensitive {
		t.Fatalf("expected nested matcher to be case-insensitive: %#v", nested)
	}
}

func TestLoadRootBuildsURLPathTemplateAndPathParameters(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/path.json", pathTemplateMapping())

	items, err := NewLoader(false).LoadRoot(root)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	request := items[0].Request
	if request.URLKind != mapping.URLMatchKindURLPathTemplate || request.URLValue == "" {
		t.Fatalf("unexpected URL matcher: %#v", request)
	}
	if request.PathParameters["contactId"].Operator != mapping.OperatorEqualTo {
		t.Fatalf("unexpected path parameters: %#v", request.PathParameters)
	}
}

func TestLoadRootRejectsInvalidURLPathTemplate(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", invalidPathTemplateMapping())

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request.urlPathTemplate", "full-segment")
}

func TestLoadRootRejectsUnsupportedIncludesNestedOperator(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", unsupportedNestedIncludesMapping())

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request.queryParameters.syscode.includes[0]", "unsupported operator before")
}

func TestLoadRootRejectsCompositeCaseInsensitive(t *testing.T) {
	root := newMockRoot(t)
	writeFile(t, root, "mappings/bad.yaml", outerCaseInsensitiveIncludesMapping())

	err := loadRootError(root)

	assertErrorContains(t, err, "bad.yaml", "request.queryParameters.syscode.caseInsensitive", "only supported")
}

func assertHasExactlyMatcher(t *testing.T, matcher mapping.Matcher, value string) {
	t.Helper()
	if matcher.Operator != mapping.OperatorHasExactly {
		t.Fatalf("expected hasExactly, got %#v", matcher)
	}
	if len(matcher.ValueMatchers) != 1 {
		t.Fatalf("expected one nested matcher, got %#v", matcher)
	}
	nested := matcher.ValueMatchers[0]
	if nested.Operator != mapping.OperatorEqualTo || nested.Value != value {
		t.Fatalf("unexpected nested matcher: %#v", nested)
	}
}

func assertIncludesMatcher(t *testing.T, matcher mapping.Matcher, value string) {
	t.Helper()
	if matcher.Operator != mapping.OperatorIncludes {
		t.Fatalf("expected includes, got %#v", matcher)
	}
	if len(matcher.ValueMatchers) != 1 {
		t.Fatalf("expected one nested matcher, got %#v", matcher)
	}
	nested := matcher.ValueMatchers[0]
	if nested.Operator != mapping.OperatorEqualTo || nested.Value != value {
		t.Fatalf("unexpected nested matcher: %#v", nested)
	}
}

func hasExactlyMapping() string {
	return `{
	  request: {
	    urlPath: '/search',
	    queryParameters: {syscode: {hasExactly: [{equalTo: 'UFONA'}]}},
	    headers: {'X-System': {hasExactly: [{equalTo: 'UFONA'}]}},
	  },
	  response: {status: 200},
	}`
}

func unsupportedNestedHasExactlyMapping() string {
	return "request:\n  urlPath: /search\n  queryParameters:\n    syscode:\n      hasExactly:\n        - before: x\nresponse:\n  status: 200\n"
}

func includesMapping() string {
	return `{
	  request: {
	    urlPath: '/search',
	    queryParameters: {syscode: {includes: [{equalTo: 'UFONA'}]}},
	    headers: {'X-System': {includes: [{equalTo: 'UFONA'}]}},
	    cookies: {session: {includes: [{equalTo: 'UFONA'}]}},
	  },
	  response: {status: 200},
	}`
}

func nestedCaseInsensitiveIncludesMapping() string {
	return `{
	  request: {
	    urlPath: '/search',
	    queryParameters: {syscode: {includes: [{equalTo: 'UFONA', caseInsensitive: true}]}},
	  },
	  response: {status: 200},
	}`
}

func pathTemplateMapping() string {
	return `{
	  request: {
	    urlPathTemplate: '/contacts/{contactId}/addresses/{addressId}',
	    pathParameters: {contactId: {equalTo: 'c123'}, addressId: {matches: '^a\\d+$'}},
	  },
	  response: {status: 200},
	}`
}

func invalidPathTemplateMapping() string {
	return "request:\n  urlPathTemplate: /contacts/{contactId/details\nresponse:\n  status: 200\n"
}

func unsupportedNestedIncludesMapping() string {
	return "request:\n  urlPath: /search\n  queryParameters:\n    syscode:\n      includes:\n        - before: x\nresponse:\n  status: 200\n"
}

func outerCaseInsensitiveIncludesMapping() string {
	return "request:\n  urlPath: /search\n  queryParameters:\n    syscode:\n      includes:\n        - equalTo: UFONA\n      caseInsensitive: true\nresponse:\n  status: 200\n"
}
