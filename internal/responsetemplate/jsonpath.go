package responsetemplate

import (
	"encoding/json"
	"fmt"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

func jsonPathValue(source any, expression string) (any, error) {
	parsed, err := parseJSONSource(source)
	if err != nil {
		return nil, err
	}
	path, err := jp.ParseString(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid JSONPath: %w", err)
	}
	return singleOrMany(path.Get(parsed)), nil
}

func parseJSONSource(source any) (any, error) {
	switch value := source.(type) {
	case []byte:
		return parseJSONBytes(value)
	case string:
		return parseJSONBytes([]byte(value))
	default:
		return source, nil
	}
}

func parseJSONBytes(data []byte) (any, error) {
	parsed, err := oj.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return parsed, nil
}

func singleOrMany(values []any) any {
	if len(values) == 0 {
		return nil
	}
	if len(values) == 1 {
		return values[0]
	}
	return values
}

func stringify(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	case bool, int, int64, float64:
		return fmt.Sprint(typed), nil
	default:
		encoded, err := json.Marshal(typed)
		return string(encoded), err
	}
}
