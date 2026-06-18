package matcher

import (
	"strings"
	"testing"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func TestRequestMatcherMatchesMethodCaseInsensitive(t *testing.T) {
	item := newMapping("get-users", 1, mapping.Request{Method: "GET", URLKind: mapping.URLMatchKindURLPath, URLValue: "/users"})

	result := Match(item, Request{Method: "get", URI: "/users"})

	assertMatched(t, result)
}

func TestRequestMatcherMatchesHeaderNamesCaseInsensitive(t *testing.T) {
	item := newMapping("secure", 1, mapping.Request{URLKind: mapping.URLMatchKindURLPath, URLValue: "/secure", Headers: map[string]mapping.Matcher{
		"Authorization": {Operator: mapping.OperatorContains, Value: "Bearer"},
	}})

	result := Match(item, Request{Method: "GET", URI: "/secure", Headers: map[string][]string{"authorization": {"Bearer token"}}})

	assertMatched(t, result)
}

func TestRequestMatcherURLSemantics(t *testing.T) {
	tests := []struct {
		name    string
		request mapping.Request
		uri     string
		want    bool
	}{
		{name: "url includes exact query", request: urlRequest(mapping.URLMatchKindURL, "/users?active=true"), uri: "/users?active=true", want: true},
		{name: "url rejects missing query", request: urlRequest(mapping.URLMatchKindURL, "/users?active=true"), uri: "/users", want: false},
		{name: "urlPath ignores query", request: urlRequest(mapping.URLMatchKindURLPath, "/users"), uri: "/users?active=true", want: true},
		{name: "urlPattern matches request uri", request: urlRequest(mapping.URLMatchKindURLPattern, `^/users/\d+$`), uri: "/users/42", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(newMapping("url", 1, tt.request), Request{URI: tt.uri})
			if result.Matched != tt.want {
				t.Fatalf("expected matched=%t, got %#v", tt.want, result)
			}
		})
	}
}

func TestRequestMatcherNamedOperators(t *testing.T) {
	item := newMapping("filters", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/search",
		Headers: map[string]mapping.Matcher{
			"X-Trace": {Operator: mapping.OperatorMatches, Value: `^trace-[0-9]+$`},
			"X-Debug": {Operator: mapping.OperatorAbsent},
		},
		QueryParameters: map[string]mapping.Matcher{
			"q":       {Operator: mapping.OperatorContains, Value: "john"},
			"version": {Operator: mapping.OperatorEqualTo, Value: "1"},
			"unused":  {Operator: mapping.OperatorAbsent},
		},
	})

	request := Request{URI: "/search?q=johnny&version=1", Headers: map[string][]string{"x-trace": {"trace-123"}}}

	assertMatched(t, Match(item, request))
}

func TestRequestMatcherReportsRegexFailure(t *testing.T) {
	item := newMapping("bad-regex", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/search",
		Headers:  map[string]mapping.Matcher{"X-ID": {Operator: mapping.OperatorMatches, Value: "["}},
	})

	result := Match(item, Request{URI: "/search", Headers: map[string][]string{"X-ID": {"abc"}}})

	assertUnmatchedReason(t, result, "invalid regex")
}

func TestRequestMatcherBodyPatterns(t *testing.T) {
	item := newMapping("body", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/users",
		BodyPatterns: []mapping.Matcher{
			{Operator: mapping.OperatorEqualTo, Value: `{"user":{"id":42,"active":true}}`},
			{Operator: mapping.OperatorContains, Value: "active"},
			{Operator: mapping.OperatorMatchesJSONPath, Value: "$.user.id"},
		},
	})

	result := Match(item, Request{URI: "/users", Body: []byte(`{"user":{"id":42,"active":true}}`)})

	assertMatched(t, result)
}

func TestRequestMatcherReportsUnmatchedReason(t *testing.T) {
	item := newMapping("reason", 1, mapping.Request{Method: "POST", URLKind: mapping.URLMatchKindURLPath, URLValue: "/orders"})

	result := Match(item, Request{Method: "GET", URI: "/orders"})

	assertUnmatchedReason(t, result, "method")
}

func TestSelectDeterministicOrdering(t *testing.T) {
	items := []mapping.Mapping{
		newMapping("b", 5, urlRequest(mapping.URLMatchKindURLPath, "/users")),
		newMapping("low-priority", 10, urlRequest(mapping.URLMatchKindURL, "/users?active=true")),
		newMapping("a", 5, urlRequest(mapping.URLMatchKindURLPath, "/users")),
		newMapping("specific", 5, mapping.Request{
			URLKind:         mapping.URLMatchKindURLPath,
			URLValue:        "/users",
			QueryParameters: map[string]mapping.Matcher{"active": {Operator: mapping.OperatorEqualTo, Value: "true"}},
		}),
	}

	selection := Select(items, Request{URI: "/users?active=true"})

	if !selection.Found() {
		t.Fatalf("expected a selected mapping, got %#v", selection)
	}
	if selection.Mapping.ID != "specific" {
		t.Fatalf("expected specificity to win within priority, got %q", selection.Mapping.ID)
	}

	selection = Select(items[:3], Request{URI: "/users?active=true"})
	if selection.Mapping.ID != "a" {
		t.Fatalf("expected id tie-breaker to win, got %q", selection.Mapping.ID)
	}
}

func newMapping(id string, priority int, request mapping.Request) mapping.Mapping {
	return mapping.Mapping{ID: id, Priority: priority, Request: request}
}

func urlRequest(kind mapping.URLMatchKind, value string) mapping.Request {
	return mapping.Request{URLKind: kind, URLValue: value}
}

func assertMatched(t *testing.T, result MatchResult) {
	t.Helper()
	if !result.Matched {
		t.Fatalf("expected match, got %#v", result)
	}
	if result.Score <= 0 {
		t.Fatalf("expected positive score, got %#v", result)
	}
}

func assertUnmatchedReason(t *testing.T, result MatchResult, reasonPart string) {
	t.Helper()
	if result.Matched {
		t.Fatalf("expected no match, got %#v", result)
	}
	if !strings.Contains(result.Reason, reasonPart) {
		t.Fatalf("expected reason containing %q, got %#v", reasonPart, result)
	}
}
