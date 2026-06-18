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
