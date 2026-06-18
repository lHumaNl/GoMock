package responsetemplate

import (
	"fmt"
	"strings"
)

const defaultEachItemName = "this"

func findBlock(template string, name string) (string, string, error) {
	depth := 1
	position := 0
	for position < len(template) {
		start, close, action, ok := findActionAt(template, position)
		if !ok {
			break
		}
		depth = blockDepth(depth, action, name)
		if depth == 0 {
			body := template[:start]
			return body, template[close+len(templateClose):], nil
		}
		position = close + len(templateClose)
	}
	return "", "", fmt.Errorf("missing closing action for %s block", name)
}

func findActionAt(template string, position int) (int, int, string, bool) {
	startOffset := strings.Index(template[position:], templateOpen)
	if startOffset < 0 {
		return 0, 0, "", false
	}
	start := position + startOffset
	closeOffset := strings.Index(template[start+len(templateOpen):], templateClose)
	if closeOffset < 0 {
		return 0, 0, "", false
	}
	close := start + len(templateOpen) + closeOffset
	action := strings.TrimSpace(template[start+len(templateOpen) : close])
	return start, close, action, true
}

func blockDepth(depth int, action string, name string) int {
	if strings.HasPrefix(action, "#"+name) {
		return depth + 1
	}
	if action == "/"+name {
		return depth - 1
	}
	return depth
}

func eachItems(action string, ctx context) ([]any, string, error) {
	expression, name, err := parseEachAction(action)
	if err != nil {
		return nil, "", err
	}
	value, err := evalExpression(expression, ctx)
	if err != nil {
		return nil, "", err
	}
	return toItems(value), name, nil
}

func parseEachAction(action string) (string, string, error) {
	content := strings.TrimSpace(strings.TrimPrefix(action, "#"+eachBlock))
	expression, rest, err := parenthesized(content)
	if err != nil {
		return "", "", err
	}
	return expression, eachAlias(rest), nil
}

func parenthesized(input string) (string, string, error) {
	if !strings.HasPrefix(input, "(") {
		return "", "", fmt.Errorf("each requires a parenthesized expression")
	}
	end := matchingParen(input)
	if end < 0 {
		return "", "", fmt.Errorf("each expression is missing closing parenthesis")
	}
	return input[1:end], strings.TrimSpace(input[end+1:]), nil
}

func matchingParen(input string) int {
	quote := rune(0)
	depth := 0
	for index, char := range input {
		quote = nextQuote(quote, char)
		if quote != 0 {
			continue
		}
		depth += parenDelta(char)
		if depth == 0 {
			return index
		}
	}
	return -1
}

func nextQuote(current rune, char rune) rune {
	if current == char {
		return 0
	}
	if current == 0 && (char == '\'' || char == '"') {
		return char
	}
	return current
}

func parenDelta(char rune) int {
	if char == '(' {
		return 1
	}
	if char == ')' {
		return -1
	}
	return 0
}

func eachAlias(input string) string {
	fields := strings.Fields(input)
	if len(fields) == 3 && fields[0] == "as" && fields[1] == "|" && fields[2] == "|" {
		return defaultEachItemName
	}
	if len(fields) == 2 && fields[0] == "as" {
		return strings.Trim(fields[1], "|")
	}
	return defaultEachItemName
}

func toItems(value any) []any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []any:
		return typed
	default:
		return []any{typed}
	}
}

func unlessCondition(action string, ctx context) bool {
	condition := strings.TrimSpace(strings.TrimPrefix(action, "#"+unlessBlock))
	return condition == lastRef && ctx.last
}
