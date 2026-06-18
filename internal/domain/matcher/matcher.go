package matcher

import (
	"bytes"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/wiremockregex"
	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

const (
	methodScore          = 10
	exactURLScore        = 1000
	urlPathScore         = 900
	urlPathTemplateScore = 700
	urlPatternScore      = 600
	namedMatcherScore    = 10
	bodyMatcherScore     = 20
)

type Request struct {
	Method  string
	URI     string
	Headers map[string][]string
	Cookies map[string][]string
	Body    []byte
}

type MatchResult struct {
	Matched bool
	Reason  string
	Score   int
}

type Selection struct {
	Mapping   *mapping.Mapping
	Match     MatchResult
	Unmatched []MatchResult
	Index     int
}

func (s Selection) Found() bool {
	return s.Mapping != nil && s.Match.Matched
}

func Match(item mapping.Mapping, request Request) MatchResult {
	ctx := matchContext{item: item, request: request, uri: requestURI(request.URI)}
	return ctx.match()
}

func Select(items []mapping.Mapping, request Request) Selection {
	candidates, unmatched := matchingCandidates(items, request)
	if len(candidates) == 0 {
		return Selection{Unmatched: unmatched}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].less(candidates[j])
	})
	best := candidates[0]
	return Selection{Mapping: &items[best.index], Match: best.result, Unmatched: unmatched, Index: best.index}
}

type candidate struct {
	index  int
	item   mapping.Mapping
	result MatchResult
}

func (c candidate) less(other candidate) bool {
	if c.item.Priority != other.item.Priority {
		return c.item.Priority < other.item.Priority
	}
	if c.result.Score != other.result.Score {
		return c.result.Score > other.result.Score
	}
	return c.item.ID < other.item.ID
}

func matchingCandidates(items []mapping.Mapping, request Request) ([]candidate, []MatchResult) {
	candidates := make([]candidate, 0, len(items))
	unmatched := make([]MatchResult, 0)
	for index := range items {
		result := Match(items[index], request)
		if result.Matched {
			candidates = append(candidates, candidate{index: index, item: items[index], result: result})
			continue
		}
		unmatched = append(unmatched, result)
	}
	return candidates, unmatched
}

type matchContext struct {
	item           mapping.Mapping
	request        Request
	uri            normalizedURI
	pathParameters map[string]string
	score          int
}

func (c *matchContext) match() MatchResult {
	if result := c.matchMethod(); !result.Matched {
		return result
	}
	if result := c.matchURL(); !result.Matched {
		return result
	}
	if result := c.matchNamed("header", c.item.Request.Headers, c.headerValues); !result.Matched {
		return result
	}
	if result := c.matchNamed("query", c.item.Request.QueryParameters, c.queryValues); !result.Matched {
		return result
	}
	if result := c.matchNamed("cookie", c.item.Request.Cookies, c.cookieValues); !result.Matched {
		return result
	}
	if result := c.matchNamed("path parameter", c.item.Request.PathParameters, c.pathParameterValues); !result.Matched {
		return result
	}
	if result := c.matchBasicAuth(); !result.Matched {
		return result
	}
	return c.matchBody()
}

func (c *matchContext) matched() MatchResult {
	return MatchResult{Matched: true, Score: c.score}
}

func (c *matchContext) unmatched(reason string) MatchResult {
	return MatchResult{Reason: reason, Score: c.score}
}

func (c *matchContext) matchMethod() MatchResult {
	expected := strings.TrimSpace(c.item.Request.Method)
	if expected == "" {
		return c.matched()
	}
	if !strings.EqualFold(expected, c.request.Method) {
		return c.unmatched(fmt.Sprintf("method expected %s", strings.ToUpper(expected)))
	}
	c.score += methodScore
	return c.matched()
}

func (c *matchContext) matchURL() MatchResult {
	switch c.item.Request.URLKind {
	case mapping.URLMatchKindURL:
		return c.matchExactURL()
	case mapping.URLMatchKindURLPath:
		return c.matchURLPath()
	case mapping.URLMatchKindURLPathTemplate:
		return c.matchURLPathTemplate()
	case mapping.URLMatchKindURLPattern:
		return c.matchURLPattern()
	case mapping.URLMatchKindURLPathPattern:
		return c.matchURLPathPattern()
	default:
		return c.unmatched("url matcher is not configured")
	}
}

func (c *matchContext) matchExactURL() MatchResult {
	if c.uri.value != c.item.Request.URLValue {
		return c.unmatched(fmt.Sprintf("url expected %s", c.item.Request.URLValue))
	}
	c.score += exactURLScore
	return c.matched()
}

func (c *matchContext) matchURLPath() MatchResult {
	if c.uri.path != c.item.Request.URLValue {
		return c.unmatched(fmt.Sprintf("urlPath expected %s", c.item.Request.URLValue))
	}
	c.score += urlPathScore
	return c.matched()
}

func (c *matchContext) matchURLPathTemplate() MatchResult {
	params, templateScore, ok := matchPathTemplate(c.item.Request.URLValue, c.uri.path)
	if !ok {
		return c.unmatched(fmt.Sprintf("urlPathTemplate expected %s", c.item.Request.URLValue))
	}
	c.pathParameters = params
	c.score += urlPathTemplateScore + templateScore
	return c.matched()
}

func (c *matchContext) matchURLPattern() MatchResult {
	matched, err := wiremockregex.MatchString(c.item.Request.URLValue, c.uri.value)
	if err != nil {
		return c.unmatched("urlPattern has invalid regex")
	}
	if !matched {
		return c.unmatched(fmt.Sprintf("urlPattern expected %s", c.item.Request.URLValue))
	}
	c.score += urlPatternScore
	return c.matched()
}

func (c *matchContext) matchURLPathPattern() MatchResult {
	matched, err := wiremockregex.MatchString(c.item.Request.URLValue, c.uri.path)
	if err != nil {
		return c.unmatched("urlPathPattern has invalid regex")
	}
	if !matched {
		return c.unmatched(fmt.Sprintf("urlPathPattern expected %s", c.item.Request.URLValue))
	}
	c.score += urlPatternScore
	return c.matched()
}

func (c *matchContext) matchNamed(
	kind string,
	matchers map[string]mapping.Matcher,
	values func(string) []string,
) MatchResult {
	for _, name := range sortedMatcherNames(matchers) {
		if result := c.matchValue(kind, name, matchers[name], values(name)); !result.Matched {
			return result
		}
		c.score += namedMatcherScore
	}
	return c.matched()
}

func (c *matchContext) matchValue(kind string, name string, matcher mapping.Matcher, values []string) MatchResult {
	matched, reason := matchValues(matcher, values)
	if matched {
		return c.matched()
	}
	return c.unmatched(fmt.Sprintf("%s %s %s", kind, name, reason))
}

func (c *matchContext) matchBody() MatchResult {
	for index, matcher := range c.item.Request.BodyPatterns {
		matched, reason := matchBodyPattern(matcher, c.request.Body)
		if !matched {
			return c.unmatched(fmt.Sprintf("bodyPatterns[%d] %s", index, reason))
		}
		c.score += bodyMatcherScore
	}
	return c.matched()
}

func (c *matchContext) headerValues(name string) []string {
	for key, values := range c.request.Headers {
		if strings.EqualFold(key, name) {
			return values
		}
	}
	return nil
}

func (c *matchContext) queryValues(name string) []string {
	return c.uri.query[name]
}

func (c *matchContext) cookieValues(name string) []string {
	if values := c.request.Cookies[name]; len(values) > 0 {
		return values
	}
	return cookieHeaderValues(c.headerValues("Cookie"), name)
}

func (c *matchContext) pathParameterValues(name string) []string {
	value, ok := c.pathParameters[name]
	if !ok {
		return nil
	}
	return []string{value}
}

func (c *matchContext) matchBasicAuth() MatchResult {
	if c.item.Request.BasicAuth == nil {
		return c.matched()
	}
	username, password, ok := parseBasicAuth(c.headerValues("Authorization"))
	if !ok || username != c.item.Request.BasicAuth.Username || password != c.item.Request.BasicAuth.Password {
		return c.unmatched("basicAuth expected configured credentials")
	}
	c.score += namedMatcherScore
	return c.matched()
}

func sortedMatcherNames(matchers map[string]mapping.Matcher) []string {
	names := make([]string, 0, len(matchers))
	for name := range matchers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func matchValues(matcher mapping.Matcher, values []string) (bool, string) {
	switch matcher.Operator {
	case mapping.OperatorAbsent:
		return len(values) == 0, "expected absent"
	case mapping.OperatorEqualTo:
		return anyValue(values, func(value string) bool { return equalValue(value, matcher) }), "expected equalTo"
	case mapping.OperatorContains:
		return anyValue(values, func(value string) bool { return containsValue(value, matcher) }), "expected contains"
	case mapping.OperatorDoesNotContain:
		return noValue(values, func(value string) bool { return containsValue(value, matcher) }), "expected doesNotContain"
	case mapping.OperatorMatches:
		return matchRegexValues(values, regexExpression(matcher))
	case mapping.OperatorDoesNotMatch:
		return notMatchRegexValues(values, regexExpression(matcher))
	case mapping.OperatorHasExactly:
		return matchExactlyValues(matcher.ValueMatchers, values)
	case mapping.OperatorIncludes:
		return matchIncludedValues(matcher.ValueMatchers, values)
	default:
		return false, "has unsupported operator"
	}
}

func matchExactlyValues(matchers []mapping.Matcher, values []string) (bool, string) {
	if len(values) != len(matchers) {
		return false, "expected hasExactly"
	}
	used := make([]bool, len(values))
	for _, matcher := range matchers {
		index := exactValueMatchIndex(matcher, values, used)
		if index < 0 {
			return false, "expected hasExactly"
		}
		used[index] = true
	}
	return true, "expected hasExactly"
}

func exactValueMatchIndex(matcher mapping.Matcher, values []string, used []bool) int {
	for index, value := range values {
		if used[index] {
			continue
		}
		matched, _ := matchValues(matcher, []string{value})
		if matched {
			return index
		}
	}
	return -1
}

func matchIncludedValues(matchers []mapping.Matcher, values []string) (bool, string) {
	for _, matcher := range matchers {
		if !anyValueMatches(matcher, values) {
			return false, "expected includes"
		}
	}
	return true, "expected includes"
}

func anyValueMatches(matcher mapping.Matcher, values []string) bool {
	for _, value := range values {
		matched, _ := matchValues(matcher, []string{value})
		if matched {
			return true
		}
	}
	return false
}

func matchBodyPattern(matcher mapping.Matcher, body []byte) (bool, string) {
	content := string(body)
	switch matcher.Operator {
	case mapping.OperatorContains:
		return containsValue(content, matcher), "expected contains"
	case mapping.OperatorDoesNotContain:
		return !containsValue(content, matcher), "expected doesNotContain"
	case mapping.OperatorEqualTo:
		return equalValue(content, matcher), "expected equalTo"
	case mapping.OperatorMatches:
		return matchRegexValues([]string{content}, regexExpression(matcher))
	case mapping.OperatorDoesNotMatch:
		return notMatchRegexValues([]string{content}, regexExpression(matcher))
	case mapping.OperatorMatchesJSONPath:
		return jsonPathExists(matcher.Value, body)
	case mapping.OperatorMatchesXPath:
		return xmlPathExists(matcher.Value, body)
	default:
		return false, "has unsupported operator"
	}
}

func jsonPathExists(expression string, body []byte) (bool, string) {
	// ojg keeps JSONPath evaluation in pure Go and avoids coupling this domain
	// matcher package to HTTP, filesystem, or runtime server concerns.
	parsed, err := oj.Parse(body)
	if err != nil {
		return false, "contains invalid JSON"
	}
	path, err := jp.ParseString(expression)
	if err != nil {
		return false, "has invalid JSONPath"
	}
	return len(path.Get(parsed)) > 0, "expected matchesJsonPath"
}

func xmlPathExists(expression string, body []byte) (bool, string) {
	// xmlquery provides XPath 1.0 node-set evaluation, including common
	// WireMock SOAP predicates such as local-name() and normalize-space().
	document, err := xmlquery.Parse(bytes.NewReader(body))
	if err != nil {
		return false, "contains invalid XML"
	}
	nodes, err := xmlquery.QueryAll(document, expression)
	if err != nil {
		return false, "has invalid XPath"
	}
	return len(nodes) > 0, "expected matchesXPath"
}

type normalizedURI struct {
	value string
	path  string
	query url.Values
}

func requestURI(raw string) normalizedURI {
	parsed, err := url.Parse(raw)
	if err != nil {
		return normalizedURI{value: raw, path: raw, query: url.Values{}}
	}
	value := requestTarget(parsed, raw)
	return normalizedURI{value: value, path: parsed.EscapedPath(), query: parsed.Query()}
}

func requestTarget(parsed *url.URL, raw string) string {
	if parsed.IsAbs() {
		return parsed.RequestURI()
	}
	if raw == "" {
		return "/"
	}
	return raw
}
