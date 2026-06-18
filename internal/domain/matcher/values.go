package matcher

import (
	"strings"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/wiremockregex"
)

func matchRegexValues(values []string, expression string) (bool, string) {
	compiled, err := wiremockregex.Compile(expression)
	if err != nil {
		return false, "has invalid regex"
	}
	return anyRegexValue(values, compiled)
}

func anyRegexValue(values []string, compiled *wiremockregex.Regex) (bool, string) {
	for _, value := range values {
		matched, err := compiled.MatchString(value)
		if err != nil {
			return false, "has invalid regex"
		}
		if matched {
			return true, "expected matches"
		}
	}
	return false, "expected matches"
}

func notMatchRegexValues(values []string, expression string) (bool, string) {
	matched, reason := matchRegexValues(values, expression)
	if reason == "has invalid regex" {
		return false, reason
	}
	return !matched, "expected doesNotMatch"
}

func regexExpression(matcher mapping.Matcher) string {
	if matcher.CaseInsensitive {
		return "(?i)" + matcher.Value
	}
	return matcher.Value
}

func equalValue(value string, matcher mapping.Matcher) bool {
	return comparableValue(value, matcher) == comparableValue(matcher.Value, matcher)
}

func containsValue(value string, matcher mapping.Matcher) bool {
	return strings.Contains(comparableValue(value, matcher), comparableValue(matcher.Value, matcher))
}

func comparableValue(value string, matcher mapping.Matcher) string {
	if matcher.CaseInsensitive {
		return strings.ToLower(value)
	}
	return value
}

func anyValue(values []string, match func(string) bool) bool {
	for _, value := range values {
		if match(value) {
			return true
		}
	}
	return false
}

func noValue(values []string, match func(string) bool) bool {
	for _, value := range values {
		if match(value) {
			return false
		}
	}
	return true
}
