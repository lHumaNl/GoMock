package configloader

import (
	"fmt"
	"strconv"

	"github.com/antchfx/xpath"
	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func validateRequestOperators(path string, raw *rawRequest) error {
	if err := validateBasicAuthShape(path, raw); err != nil {
		return err
	}
	if err := validatePathParametersShape(path, raw); err != nil {
		return err
	}
	if err := validateNamedOperators(path, "request.headers", raw.Headers, headerQueryOperators); err != nil {
		return err
	}
	if err := validateNamedOperators(path, "request.queryParameters", raw.QueryParameters, headerQueryOperators); err != nil {
		return err
	}
	if err := validateNamedOperators(path, "request.cookies", raw.Cookies, headerQueryOperators); err != nil {
		return err
	}
	if err := validateNamedOperators(path, "request.pathParameters", raw.PathParameters, pathParameterOperators); err != nil {
		return err
	}
	return validateBodyOperators(path, raw.BodyPatterns)
}

func validatePathParametersShape(path string, raw *rawRequest) error {
	if len(raw.PathParameters) > 0 && raw.URLPathTemplate == "" {
		return configError(path, "request.pathParameters", "requires request.urlPathTemplate")
	}
	return nil
}

func validateBasicAuthShape(path string, raw *rawRequest) error {
	if raw.BasicAuth != nil && raw.BasicAuthCredentials != nil {
		return configError(path, "request.basicAuth", "is mutually exclusive with basicAuthCredentials")
	}
	return nil
}

func validateNamedOperators(path string, field string, values map[string]rawOperator, allowed map[string]struct{}) error {
	for name, operator := range values {
		if err := validateOperator(path, field+"."+name, operator, allowed); err != nil {
			return err
		}
	}
	return nil
}

func validateBodyOperators(path string, patterns []rawOperator) error {
	for index, operator := range patterns {
		field := "request.bodyPatterns[" + itoa(index) + "]"
		if err := validateOperator(path, field, operator, bodyOperators); err != nil {
			return err
		}
	}
	return nil
}

func validateOperator(path string, field string, operator rawOperator, allowed map[string]struct{}) error {
	operatorCount := countOperators(operator)
	if operatorCount != 1 {
		return configError(path, field, "requires exactly one operator")
	}
	name, value := singleOperator(operator)
	if _, ok := allowed[name]; !ok {
		return configError(path, field, "has unsupported operator "+name)
	}
	operatorName := mapping.Operator(name)
	if err := validateCaseInsensitive(path, field, operator, operatorName); err != nil {
		return err
	}
	return validateOperatorValue(path, field, operatorName, value)
}

func singleOperator(operator rawOperator) (string, any) {
	for name, value := range operator {
		if name != "caseInsensitive" {
			return name, value
		}
	}
	return "", nil
}

func countOperators(operator rawOperator) int {
	count := 0
	for name := range operator {
		if name != "caseInsensitive" {
			count++
		}
	}
	return count
}

func validateCaseInsensitive(path string, field string, operator rawOperator, name mapping.Operator) error {
	value, ok := operator["caseInsensitive"]
	if !ok {
		return nil
	}
	if _, ok := value.(bool); !ok {
		return configError(path, field+".caseInsensitive", "must be boolean")
	}
	if !supportsCaseInsensitive(name) {
		return configError(path, field+".caseInsensitive", "is only supported with equalTo, contains, or doesNotContain")
	}
	return nil
}

func supportsCaseInsensitive(name mapping.Operator) bool {
	return name == mapping.OperatorEqualTo ||
		name == mapping.OperatorContains ||
		name == mapping.OperatorDoesNotContain
}

func validateOperatorValue(path string, field string, operator mapping.Operator, value any) error {
	if operator == mapping.OperatorHasExactly || operator == mapping.OperatorIncludes {
		return validateNestedValueMatchers(path, field+"."+string(operator), value)
	}
	if operator == mapping.OperatorMatches || operator == mapping.OperatorDoesNotMatch {
		return validateRegex(path, field+"."+string(operator), stringValue(value))
	}
	if operator == mapping.OperatorMatchesXPath {
		return validateXPath(path, field+".matchesXPath", stringValue(value))
	}
	return nil
}

func validateNestedValueMatchers(path string, field string, value any) error {
	items, ok := value.([]any)
	if !ok {
		return configError(path, field, "must be an array of value matchers")
	}
	for index, item := range items {
		if err := validateNestedValueMatcher(path, field, index, item); err != nil {
			return err
		}
	}
	return nil
}

func validateNestedValueMatcher(path string, field string, index int, item any) error {
	itemField := field + "[" + itoa(index) + "]"
	operator, ok := rawOperatorFromAny(item)
	if !ok {
		return configError(path, itemField, "must be a value matcher object")
	}
	return validateOperator(path, itemField, operator, nestedValueOperators)
}

func validateXPath(path string, field string, expression string) error {
	if _, err := xpath.Compile(expression); err != nil {
		return configError(path, field, "has invalid XPath")
	}
	return nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

var headerQueryOperators = map[string]struct{}{
	string(mapping.OperatorEqualTo):        {},
	string(mapping.OperatorContains):       {},
	string(mapping.OperatorMatches):        {},
	string(mapping.OperatorDoesNotMatch):   {},
	string(mapping.OperatorDoesNotContain): {},
	string(mapping.OperatorAbsent):         {},
	string(mapping.OperatorHasExactly):     {},
	string(mapping.OperatorIncludes):       {},
}

var pathParameterOperators = map[string]struct{}{
	string(mapping.OperatorEqualTo):        {},
	string(mapping.OperatorContains):       {},
	string(mapping.OperatorMatches):        {},
	string(mapping.OperatorDoesNotMatch):   {},
	string(mapping.OperatorDoesNotContain): {},
	string(mapping.OperatorAbsent):         {},
}

var nestedValueOperators = map[string]struct{}{
	string(mapping.OperatorEqualTo):        {},
	string(mapping.OperatorContains):       {},
	string(mapping.OperatorMatches):        {},
	string(mapping.OperatorDoesNotMatch):   {},
	string(mapping.OperatorDoesNotContain): {},
}

var bodyOperators = map[string]struct{}{
	string(mapping.OperatorMatches):         {},
	string(mapping.OperatorDoesNotMatch):    {},
	string(mapping.OperatorEqualTo):         {},
	string(mapping.OperatorContains):        {},
	string(mapping.OperatorDoesNotContain):  {},
	string(mapping.OperatorMatchesJSONPath): {},
	string(mapping.OperatorMatchesXPath):    {},
}
