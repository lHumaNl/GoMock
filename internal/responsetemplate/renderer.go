package responsetemplate

import (
	"fmt"
	"strings"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/domain/matcher"
)

const (
	eachBlock       = "each"
	unlessBlock     = "unless"
	templateOpen    = "{{"
	templateClose   = "}}"
	originalBodyRef = "originalRequest.body"
	lastRef         = "@last"
)

type context struct {
	request matcher.Request
	vars    map[string]any
	last    bool
}

func RenderResponse(response mapping.Response, request matcher.Request) (mapping.Response, error) {
	if !response.HasTransformer(mapping.TransformerResponseTemplate) {
		return response, nil
	}
	return renderTransformedResponse(response, request)
}

func renderTransformedResponse(response mapping.Response, request matcher.Request) (mapping.Response, error) {
	ctx := context{request: request, vars: map[string]any{}}
	rendered := response
	if err := renderBody(&rendered, ctx); err != nil {
		return mapping.Response{}, err
	}
	headers, err := renderHeaders(response.Headers, ctx)
	if err != nil {
		return mapping.Response{}, err
	}
	rendered.Headers = headers
	return rendered, nil
}

func renderBody(response *mapping.Response, ctx context) error {
	if response.BodyFileName != "" {
		rendered, err := renderString(string(response.BodyFileContent), ctx)
		response.BodyFileContent = []byte(rendered)
		return err
	}
	body, err := renderString(response.Body, ctx)
	response.Body = body
	return err
}

func renderHeaders(headers map[string]string, ctx context) (map[string]string, error) {
	rendered := make(map[string]string, len(headers))
	for name, value := range headers {
		text, err := renderString(value, ctx)
		if err != nil {
			return nil, fmt.Errorf("render header %s: %w", name, err)
		}
		rendered[name] = text
	}
	return rendered, nil
}

func renderString(template string, ctx context) (string, error) {
	var builder strings.Builder
	for len(template) > 0 {
		prefix, action, suffix, ok := nextAction(template)
		builder.WriteString(prefix)
		if !ok {
			break
		}
		rendered, rest, err := renderAction(action, suffix, ctx)
		if err != nil {
			return "", err
		}
		builder.WriteString(rendered)
		template = rest
	}
	return builder.String(), nil
}

func nextAction(template string) (string, string, string, bool) {
	start := strings.Index(template, templateOpen)
	if start < 0 {
		return template, "", "", false
	}
	end := strings.Index(template[start+len(templateOpen):], templateClose)
	if end < 0 {
		return template, "", "", false
	}
	closeStart := start + len(templateOpen) + end
	action := strings.TrimSpace(template[start+len(templateOpen) : closeStart])
	return template[:start], action, template[closeStart+len(templateClose):], true
}

func renderAction(action string, suffix string, ctx context) (string, string, error) {
	switch {
	case strings.HasPrefix(action, "#"+eachBlock):
		return renderEach(action, suffix, ctx)
	case strings.HasPrefix(action, "#"+unlessBlock):
		return renderUnless(action, suffix, ctx)
	case strings.HasPrefix(action, "/"):
		return "", "", fmt.Errorf("unexpected closing action %s", action)
	default:
		return renderInlineAction(action, suffix, ctx)
	}
}

func renderInlineAction(action string, suffix string, ctx context) (string, string, error) {
	value, err := evalExpression(action, ctx)
	if err != nil {
		return "", "", err
	}
	text, err := stringify(value)
	return text, suffix, err
}

func renderEach(action string, suffix string, ctx context) (string, string, error) {
	body, rest, err := findBlock(suffix, eachBlock)
	if err != nil {
		return "", "", err
	}
	items, name, err := eachItems(action, ctx)
	if err != nil {
		return "", "", err
	}
	rendered, err := renderItems(body, items, name, ctx)
	return rendered, rest, err
}

func renderUnless(action string, suffix string, ctx context) (string, string, error) {
	body, rest, err := findBlock(suffix, unlessBlock)
	if err != nil {
		return "", "", err
	}
	if unlessCondition(action, ctx) {
		return "", rest, nil
	}
	rendered, err := renderString(body, ctx)
	return rendered, rest, err
}

func renderItems(body string, items []any, name string, ctx context) (string, error) {
	var builder strings.Builder
	for index, item := range items {
		child := ctx.withItem(name, item, index == len(items)-1)
		rendered, err := renderString(body, child)
		if err != nil {
			return "", err
		}
		builder.WriteString(rendered)
	}
	return builder.String(), nil
}

func (c context) withItem(name string, item any, last bool) context {
	vars := make(map[string]any, len(c.vars)+1)
	for key, value := range c.vars {
		vars[key] = value
	}
	vars[name] = item
	return context{request: c.request, vars: vars, last: last}
}
