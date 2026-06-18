package configloader

import (
	"fmt"
	"strconv"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func validateRequestOperators(path string, raw *rawRequest) error {
	if err := validateNamedOperators(path, "request.headers", raw.Headers, headerQueryOperators); err != nil {
		return err
	}
	if err := validateNamedOperators(path, "request.queryParameters", raw.QueryParameters, headerQueryOperators); err != nil {
		return err
	}
	return validateBodyOperators(path, raw.BodyPatterns)
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
	if len(operator) != 1 {
		return configError(path, field, "requires exactly one operator")
	}
	for name, value := range operator {
		if _, ok := allowed[name]; !ok {
			return configError(path, field, "has unsupported operator "+name)
		}
		return validateOperatorValue(path, field, mapping.Operator(name), value)
	}
	return nil
}

func validateOperatorValue(path string, field string, operator mapping.Operator, value any) error {
	if operator == mapping.OperatorMatches {
		return validateRegex(path, field+".matches", stringValue(value))
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
	string(mapping.OperatorEqualTo):  {},
	string(mapping.OperatorContains): {},
	string(mapping.OperatorMatches):  {},
	string(mapping.OperatorAbsent):   {},
}

var bodyOperators = map[string]struct{}{
	string(mapping.OperatorEqualTo):         {},
	string(mapping.OperatorContains):        {},
	string(mapping.OperatorMatchesJSONPath): {},
}
