package responsetemplate

import (
	"fmt"
	"strings"
)

const (
	jsonPathHelper    = "jsonPath"
	randomIntHelper   = "randomInt"
	randomValueHelper = "randomValue"
)

func evalExpression(expression string, ctx context) (any, error) {
	tokens, err := tokenize(strings.TrimSpace(expression))
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return "", nil
	}
	return evalTokens(tokens, ctx)
}

func evalTokens(tokens []string, ctx context) (any, error) {
	switch tokens[0] {
	case jsonPathHelper:
		return evalJSONPath(tokens, ctx)
	case randomIntHelper:
		return randomInt(namedParams(tokens[1:]))
	case randomValueHelper:
		return randomValue(namedParams(tokens[1:]))
	case lastRef:
		return ctx.last, nil
	default:
		return resolveVariable(tokens[0], ctx)
	}
}

func resolveVariable(name string, ctx context) (any, error) {
	value, ok := ctx.vars[name]
	if !ok {
		return nil, fmt.Errorf("unknown template helper or variable %s", name)
	}
	return value, nil
}

func evalJSONPath(tokens []string, ctx context) (any, error) {
	if len(tokens) != 3 {
		return nil, fmt.Errorf("jsonPath requires source and path arguments")
	}
	source, err := resolveSource(tokens[1], ctx)
	if err != nil {
		return nil, err
	}
	return jsonPathValue(source, tokens[2])
}

func resolveSource(name string, ctx context) (any, error) {
	if name == originalBodyRef {
		return ctx.request.Body, nil
	}
	value, ok := ctx.vars[name]
	if !ok {
		return nil, fmt.Errorf("unknown template source %s", name)
	}
	return value, nil
}

func namedParams(tokens []string) map[string]string {
	params := make(map[string]string, len(tokens))
	for _, token := range tokens {
		name, value, ok := strings.Cut(token, "=")
		if ok {
			params[name] = value
		}
	}
	return params
}
