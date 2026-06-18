package matcher

import (
	"net/url"
	"strings"
)

const (
	pathTemplateSegmentScore = 2
	pathTemplateStaticScore  = 3
)

type pathTemplateSegment struct {
	value     string
	parameter bool
}

func matchPathTemplate(template string, path string) (map[string]string, int, bool) {
	segments, score, ok := parsePathTemplate(template)
	if !ok || len(segments) != len(pathSegments(path)) {
		return nil, score, false
	}
	return matchTemplateSegments(segments, pathSegments(path), score)
}

func parsePathTemplate(template string) ([]pathTemplateSegment, int, bool) {
	parts := pathSegments(template)
	segments := make([]pathTemplateSegment, 0, len(parts))
	score := len(parts) * pathTemplateSegmentScore
	for _, part := range parts {
		segment, static, ok := parseTemplateSegment(part)
		if !ok {
			return nil, score, false
		}
		if static {
			score += pathTemplateStaticScore
		}
		segments = append(segments, segment)
	}
	return segments, score, true
}

func parseTemplateSegment(segment string) (pathTemplateSegment, bool, bool) {
	if !strings.Contains(segment, "{") && !strings.Contains(segment, "}") {
		return pathTemplateSegment{value: segment}, true, true
	}
	if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' {
		return pathTemplateSegment{}, false, false
	}
	name := strings.TrimSpace(segment[1 : len(segment)-1])
	return pathTemplateSegment{value: name, parameter: true}, false, name != ""
}

func matchTemplateSegments(
	template []pathTemplateSegment,
	actual []string,
	score int,
) (map[string]string, int, bool) {
	params := make(map[string]string)
	for index, segment := range template {
		if !matchTemplateSegment(segment, actual[index], params) {
			return nil, score, false
		}
	}
	return params, score, true
}

func matchTemplateSegment(segment pathTemplateSegment, actual string, params map[string]string) bool {
	if !segment.parameter {
		return segment.value == actual
	}
	value, err := url.PathUnescape(actual)
	if err != nil {
		return false
	}
	params[segment.value] = value
	return true
}

func pathSegments(path string) []string {
	return strings.Split(path, "/")
}
