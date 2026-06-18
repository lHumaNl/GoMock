package matcher

import (
	"encoding/base64"
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
		{name: "urlPathPattern matches path only", request: urlRequest(mapping.URLMatchKindURLPathPattern, `^/users/\d+$`), uri: "/users/42?q=/users/abc", want: true},
		{name: "urlPathPattern rejects path non-match", request: urlRequest(mapping.URLMatchKindURLPathPattern, `^/users/\d+$`), uri: "/users/abc?q=/users/42", want: false},
		{name: "urlPattern supports negative lookahead", request: urlRequest(mapping.URLMatchKindURLPattern, `/prweb/PRRestService/LoanMBAPI/v2/cases\?pinEq=(?!UC0).+&id=.+`), uri: "/prweb/PRRestService/LoanMBAPI/v2/cases?pinEq=UC123&id=42", want: true},
		{name: "urlPattern negative lookahead rejects excluded prefix", request: urlRequest(mapping.URLMatchKindURLPattern, `/prweb/PRRestService/LoanMBAPI/v2/cases\?pinEq=(?!UC0).+&id=.+`), uri: "/prweb/PRRestService/LoanMBAPI/v2/cases?pinEq=UC012&id=42", want: false},
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

func TestRequestMatcherQueryMatchesSupportsLookahead(t *testing.T) {
	item := newMapping("filters", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/search",
		QueryParameters: map[string]mapping.Matcher{
			"pinEq": {Operator: mapping.OperatorMatches, Value: `^(?!UC0).+`},
		},
	})

	assertMatched(t, Match(item, Request{URI: "/search?pinEq=UC123"}))
	assertUnmatchedReason(t, Match(item, Request{URI: "/search?pinEq=UC012"}), "expected matches")
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

func TestRequestMatcherNegativeAndCaseInsensitiveValueOperators(t *testing.T) {
	item := newMapping("filters", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/search",
		Headers: map[string]mapping.Matcher{
			"X-Client": {Operator: mapping.OperatorEqualTo, Value: "WEB", CaseInsensitive: true},
			"X-Debug":  {Operator: mapping.OperatorDoesNotContain, Value: "true", CaseInsensitive: true},
		},
		QueryParameters: map[string]mapping.Matcher{
			"q": {Operator: mapping.OperatorDoesNotMatch, Value: `^admin`},
		},
	})

	request := Request{URI: "/search?q=user", Headers: map[string][]string{"x-client": {"web"}, "x-debug": {"FALSE"}}}

	assertMatched(t, Match(item, request))
	assertUnmatchedReason(t, Match(item, Request{URI: "/search?q=admin-user", Headers: request.Headers}), "doesNotMatch")
}

func TestRequestMatcherCookiesAndBasicAuth(t *testing.T) {
	item := newMapping("secure", 1, mapping.Request{
		URLKind:   mapping.URLMatchKindURLPath,
		URLValue:  "/secure",
		Cookies:   map[string]mapping.Matcher{"session": {Operator: mapping.OperatorContains, Value: "abc"}},
		BasicAuth: &mapping.BasicAuth{Username: "api", Password: "secret"},
	})

	request := Request{URI: "/secure", Headers: map[string][]string{
		"Cookie":        {"theme=dark; session=abc123"},
		"Authorization": {basicAuthHeader("api", "secret")},
	}}

	assertMatched(t, Match(item, request))
	assertUnmatchedReason(t, Match(item, Request{URI: "/secure", Headers: request.Headers, Cookies: map[string][]string{"session": {"wrong"}}}), "cookie session")
}

func TestRequestMatcherQueryHasExactly(t *testing.T) {
	item := hasExactlyQueryMapping("syscode", equalToMatcher("UFONA"))
	tests := []struct {
		name string
		uri  string
		want bool
	}{
		{name: "single value matches", uri: "/search?syscode=UFONA", want: true},
		{name: "missing parameter fails", uri: "/search", want: false},
		{name: "extra value fails", uri: "/search?syscode=UFONA&syscode=EXTRA", want: false},
		{name: "wrong value fails", uri: "/search?syscode=OTHER", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(item, Request{URI: tt.uri})
			if result.Matched != tt.want {
				t.Fatalf("expected matched=%t, got %#v", tt.want, result)
			}
		})
	}
}

func TestRequestMatcherQueryHasExactlyMatchesMultipleValues(t *testing.T) {
	item := hasExactlyQueryMapping("id", equalToMatcher("1"), containsMatcher("2"))

	result := Match(item, Request{URI: "/search?id=value-2&id=1"})

	assertMatched(t, result)
}

func TestRequestMatcherNestedCaseInsensitiveIsLocal(t *testing.T) {
	item := hasExactlyQueryMapping("id", caseInsensitiveEqualToMatcher("alpha"))

	assertMatched(t, Match(item, Request{URI: "/search?id=ALPHA"}))
}

func TestRequestMatcherOuterCaseInsensitiveDoesNotPropagate(t *testing.T) {
	item := hasExactlyQueryMapping("id", equalToMatcher("alpha"))
	matcher := item.Request.QueryParameters["id"]
	matcher.CaseInsensitive = true
	item.Request.QueryParameters["id"] = matcher

	assertUnmatchedReason(t, Match(item, Request{URI: "/search?id=ALPHA"}), "hasExactly")
}

func TestRequestMatcherHeaderHasExactly(t *testing.T) {
	item := newMapping("header-values", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/things",
		Headers: map[string]mapping.Matcher{
			"X-ID": hasExactlyMatcher(equalToMatcher("1"), containsMatcher("2")),
		},
	})

	result := Match(item, Request{URI: "/things", Headers: map[string][]string{"x-id": {"1", "value-2"}}})

	assertMatched(t, result)
}

func TestRequestMatcherIncludesAllowsExtraValues(t *testing.T) {
	item := newMapping("included-values", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/things",
		Headers: map[string]mapping.Matcher{
			"X-ID": includesMatcher(equalToMatcher("1"), containsMatcher("2")),
		},
		QueryParameters: map[string]mapping.Matcher{
			"id": includesMatcher(equalToMatcher("1"), containsMatcher("2")),
		},
		Cookies: map[string]mapping.Matcher{
			"session": includesMatcher(equalToMatcher("abc"), containsMatcher("def")),
		},
	})

	request := Request{URI: "/things?id=extra&id=value-2&id=1", Headers: map[string][]string{"x-id": {"extra", "1", "value-2"}}, Cookies: map[string][]string{"session": {"abc", "prefix-def", "extra"}}}

	assertMatched(t, Match(item, request))
}

func TestRequestMatcherIncludesFailsWhenValueMissing(t *testing.T) {
	item := hasIncludesQueryMapping("id", equalToMatcher("1"), containsMatcher("missing"))

	result := Match(item, Request{URI: "/search?id=1&id=extra"})

	assertUnmatchedReason(t, result, "expected includes")
}

func TestRequestMatcherURLPathTemplateAndPathParameters(t *testing.T) {
	item := pathTemplateMapping(equalToMatcher("c123"), mapping.Matcher{Operator: mapping.OperatorMatches, Value: `^a\d+$`})

	assertMatched(t, Match(item, Request{URI: "/contacts/c123/addresses/a456?verbose=true"}))
	assertUnmatchedReason(t, Match(item, Request{URI: "/contacts/c123"}), "urlPathTemplate")
}

func TestRequestMatcherPathParametersSupportDoesNotMatch(t *testing.T) {
	item := pathTemplateMapping(mapping.Matcher{Operator: mapping.OperatorDoesNotMatch, Value: `^tmp-`}, equalToMatcher("a1"))

	assertMatched(t, Match(item, Request{URI: "/contacts/c123/addresses/a1"}))
	assertUnmatchedReason(t, Match(item, Request{URI: "/contacts/tmp-1/addresses/a1"}), "doesNotMatch")
}

func TestRequestMatcherURLPathTemplateDecodesParameters(t *testing.T) {
	item := pathTemplateMapping(equalToMatcher("c 1"), equalToMatcher("a/b"))

	assertMatched(t, Match(item, Request{URI: "/contacts/c%201/addresses/a%2Fb"}))
}

func TestSelectPrefersMoreSpecificURLPathTemplate(t *testing.T) {
	items := []mapping.Mapping{
		newMapping("generic", 5, mapping.Request{URLKind: mapping.URLMatchKindURLPathTemplate, URLValue: "/contacts/{contactId}/{resource}/{addressId}"}),
		newMapping("addresses", 5, mapping.Request{URLKind: mapping.URLMatchKindURLPathTemplate, URLValue: "/contacts/{contactId}/addresses/{addressId}"}),
	}

	selection := Select(items, Request{URI: "/contacts/c123/addresses/a456"})

	if selection.Mapping.ID != "addresses" {
		t.Fatalf("expected more specific template, got %q", selection.Mapping.ID)
	}
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
			{Operator: mapping.OperatorMatches, Value: `"id":\s*(?!0)\d+`},
			{Operator: mapping.OperatorMatchesJSONPath, Value: "$.user.id"},
		},
	})

	result := Match(item, Request{URI: "/users", Body: []byte(`{"user":{"id":42,"active":true}}`)})

	assertMatched(t, result)
}

func TestRequestMatcherNegativeBodyPatterns(t *testing.T) {
	item := newMapping("body", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/users",
		BodyPatterns: []mapping.Matcher{
			{Operator: mapping.OperatorContains, Value: "ACTIVE", CaseInsensitive: true},
			{Operator: mapping.OperatorDoesNotContain, Value: "disabled", CaseInsensitive: true},
			{Operator: mapping.OperatorDoesNotMatch, Value: `"id":\s*0`},
		},
	})

	result := Match(item, Request{URI: "/users", Body: []byte(`{"id":42,"active":true}`)})

	assertMatched(t, result)
}

func TestRequestMatcherMatchesXPathBodyPattern(t *testing.T) {
	item := xmlBodyMapping("//*[local-name()='cus' and normalize-space(text())!='']")
	body := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ns="urn:test"><soap:Body><ns:cus> C123 </ns:cus></soap:Body></soap:Envelope>`

	result := Match(item, Request{URI: "/soap", Body: []byte(body)})

	assertMatched(t, result)
}

func TestRequestMatcherReportsXPathNonMatch(t *testing.T) {
	item := xmlBodyMapping("//customer/id")

	result := Match(item, Request{URI: "/soap", Body: []byte(`<customer><name>Ada</name></customer>`)})

	assertUnmatchedReason(t, result, "expected matchesXPath")
}

func TestRequestMatcherReportsInvalidXMLForXPath(t *testing.T) {
	item := xmlBodyMapping("//customer")

	result := Match(item, Request{URI: "/soap", Body: []byte(`<customer>`)})

	assertUnmatchedReason(t, result, "invalid XML")
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

func xmlBodyMapping(expression string) mapping.Mapping {
	return newMapping("xml", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPath,
		URLValue: "/soap",
		BodyPatterns: []mapping.Matcher{
			{Operator: mapping.OperatorMatchesXPath, Value: expression},
		},
	})
}

func hasExactlyQueryMapping(name string, matchers ...mapping.Matcher) mapping.Mapping {
	return newMapping("exact-query", 1, mapping.Request{
		URLKind:         mapping.URLMatchKindURLPath,
		URLValue:        "/search",
		QueryParameters: map[string]mapping.Matcher{name: hasExactlyMatcher(matchers...)},
	})
}

func hasIncludesQueryMapping(name string, matchers ...mapping.Matcher) mapping.Mapping {
	return newMapping("includes-query", 1, mapping.Request{
		URLKind:         mapping.URLMatchKindURLPath,
		URLValue:        "/search",
		QueryParameters: map[string]mapping.Matcher{name: includesMatcher(matchers...)},
	})
}

func pathTemplateMapping(contactID mapping.Matcher, addressID mapping.Matcher) mapping.Mapping {
	return newMapping("path-template", 1, mapping.Request{
		URLKind:  mapping.URLMatchKindURLPathTemplate,
		URLValue: "/contacts/{contactId}/addresses/{addressId}",
		PathParameters: map[string]mapping.Matcher{
			"contactId": contactID,
			"addressId": addressID,
		},
	})
}

func hasExactlyMatcher(matchers ...mapping.Matcher) mapping.Matcher {
	return mapping.Matcher{Operator: mapping.OperatorHasExactly, ValueMatchers: matchers}
}

func includesMatcher(matchers ...mapping.Matcher) mapping.Matcher {
	return mapping.Matcher{Operator: mapping.OperatorIncludes, ValueMatchers: matchers}
}

func equalToMatcher(value string) mapping.Matcher {
	return mapping.Matcher{Operator: mapping.OperatorEqualTo, Value: value}
}

func containsMatcher(value string) mapping.Matcher {
	return mapping.Matcher{Operator: mapping.OperatorContains, Value: value}
}

func caseInsensitiveEqualToMatcher(value string) mapping.Matcher {
	return mapping.Matcher{Operator: mapping.OperatorEqualTo, Value: value, CaseInsensitive: true}
}

func basicAuthHeader(username string, password string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + encoded
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
