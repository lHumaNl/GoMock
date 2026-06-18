package configloader

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/files"
	"github.com/lHumaNl/gomock/internal/wiremockregex"
)

const (
	minHTTPStatus = 100
	maxHTTPStatus = 599
)

func buildMapping(path string, raw rawMapping, resolver *files.Resolver) (mapping.Mapping, error) {
	request, err := buildRequest(path, raw.Request)
	if err != nil {
		return mapping.Mapping{}, err
	}

	item := mapping.Mapping{ID: mappingID(path, raw.ID), Name: raw.Name,
		Priority: mappingPriority(raw.Priority), Request: request}
	if err := attachResponses(path, &item, raw, resolver); err != nil {
		return mapping.Mapping{}, err
	}

	return item, nil
}

func mappingID(path string, id string) string {
	if strings.TrimSpace(id) != "" {
		return id
	}
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func mappingPriority(priority *int) int {
	if priority == nil {
		return mapping.DefaultPriority
	}
	return *priority
}

func attachResponses(path string, item *mapping.Mapping, raw rawMapping, resolver *files.Resolver) error {
	hasSingle := raw.Response != nil
	hasSet := raw.Responses != nil
	if hasSingle == hasSet {
		return configError(path, "response", "requires exactly one of response or responses")
	}
	if hasSingle {
		response, err := buildResponse(path, "response", *raw.Response, resolver)
		item.Response = &response
		return err
	}
	return attachResponseSet(path, item, raw.Responses, resolver)
}

func attachResponseSet(path string, item *mapping.Mapping, raw *rawResponses, resolver *files.Resolver) error {
	mode, err := responseMode(path, raw.Mode)
	if err != nil {
		return err
	}
	if len(raw.Variants) == 0 {
		return configError(path, "responses.variants", "must contain at least one variant")
	}

	variants, err := buildVariants(path, raw, resolver)
	item.Responses = &mapping.ResponseSet{Mode: mode, Variants: variants}
	return err
}

func buildRequest(path string, raw *rawRequest) (mapping.Request, error) {
	if raw == nil {
		return mapping.Request{}, configError(path, "request", "is required")
	}

	kind, value, err := urlMatcher(path, raw)
	if err != nil {
		return mapping.Request{}, err
	}

	return mapping.Request{Method: strings.ToUpper(raw.Method), URLKind: kind, URLValue: value,
		Headers: buildNamedMatchers(raw.Headers), QueryParameters: buildNamedMatchers(raw.QueryParameters),
		BodyPatterns: buildBodyMatchers(raw.BodyPatterns)}, validateRequestOperators(path, raw)
}

func urlMatcher(path string, raw *rawRequest) (mapping.URLMatchKind, string, error) {
	values := []string{raw.URL, raw.URLPath, raw.URLPattern}
	if countNonEmpty(values) != 1 {
		return "", "", configError(path, "request.url", "requires exactly one of url, urlPath, or urlPattern")
	}
	if raw.URLPattern != "" {
		return mapping.URLMatchKindURLPattern, raw.URLPattern, validateRegex(path, "request.urlPattern", raw.URLPattern)
	}
	if raw.URLPath != "" {
		return mapping.URLMatchKindURLPath, raw.URLPath, nil
	}
	return mapping.URLMatchKindURL, raw.URL, nil
}

func buildNamedMatchers(raw map[string]rawOperator) map[string]mapping.Matcher {
	result := make(map[string]mapping.Matcher, len(raw))
	for key, operator := range raw {
		result[key] = buildMatcher(operator)
	}
	return result
}

func buildBodyMatchers(raw []rawOperator) []mapping.Matcher {
	result := make([]mapping.Matcher, 0, len(raw))
	for _, operator := range raw {
		result = append(result, buildMatcher(operator))
	}
	return result
}

func buildMatcher(raw rawOperator) mapping.Matcher {
	for name, value := range raw {
		return mapping.Matcher{Operator: mapping.Operator(name), Value: fmt.Sprint(value)}
	}
	return mapping.Matcher{}
}

func countNonEmpty(values []string) int {
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

func buildResponse(path string, field string, raw rawResponse, resolver *files.Resolver) (mapping.Response, error) {
	if err := validateResponseShape(path, field, raw); err != nil {
		return mapping.Response{}, err
	}
	delay, err := buildResponseDelay(path, field, raw)
	if err != nil {
		return mapping.Response{}, err
	}

	response := mapping.Response{Name: raw.Name, Weight: raw.Weight, Status: *raw.Status,
		Headers: raw.Headers, BodyFileName: raw.BodyFileName, Transformers: raw.Transformers, Delay: delay}
	if raw.Body != nil {
		response.Body = *raw.Body
	}
	return response, loadBodyFile(path, field, &response, resolver)
}

func buildResponseDelay(path string, field string, raw rawResponse) (*mapping.Delay, error) {
	if raw.Delay != nil && raw.DelayDistribution != nil {
		return nil, configError(path, field+".delay", "is mutually exclusive with delayDistribution")
	}
	if raw.DelayDistribution != nil {
		return buildDelayDistribution(path, field+".delayDistribution", raw.DelayDistribution)
	}
	return buildDelay(path, field+".delay", raw.Delay)
}

func validateResponseShape(path string, field string, raw rawResponse) error {
	if raw.Status == nil {
		return configError(path, field+".status", "is required")
	}
	if *raw.Status < minHTTPStatus || *raw.Status > maxHTTPStatus {
		return configError(path, field+".status", "must be between 100 and 599")
	}
	if raw.Body != nil && raw.BodyFileName != "" {
		return configError(path, field+".body", "is mutually exclusive with bodyFileName")
	}
	return nil
}

func loadBodyFile(path string, field string, response *mapping.Response, resolver *files.Resolver) error {
	if response.BodyFileName == "" {
		return nil
	}
	content, err := resolver.ReadBodyFile(response.BodyFileName)
	if err != nil {
		return configError(path, field+".bodyFileName", err.Error())
	}
	response.BodyFileContent = content
	return nil
}

func buildVariants(path string, raw *rawResponses, resolver *files.Resolver) ([]mapping.Response, error) {
	variants := make([]mapping.Response, 0, len(raw.Variants))
	for index, rawVariant := range raw.Variants {
		field := fmt.Sprintf("responses.variants[%d]", index)
		variant, err := buildResponse(path, field, rawVariant, resolver)
		if err != nil {
			return nil, err
		}
		setVariantDefaults(&variant, index)
		if err := validateVariantWeight(path, field, raw.Mode, variant.Weight); err != nil {
			return nil, err
		}
		variants = append(variants, variant)
	}
	return variants, nil
}

func setVariantDefaults(response *mapping.Response, index int) {
	if response.Name == "" {
		response.Name = fmt.Sprintf("variant-%d", index)
	}
}

func responseMode(path string, mode string) (mapping.ResponseMode, error) {
	switch mapping.ResponseMode(mode) {
	case mapping.ResponseModeSequential, mapping.ResponseModeRandom, mapping.ResponseModeWeighted:
		return mapping.ResponseMode(mode), nil
	default:
		return "", configError(path, "responses.mode", "must be sequential, random, or weighted")
	}
}

func validateVariantWeight(path string, field string, mode string, weight int) error {
	if mapping.ResponseMode(mode) == mapping.ResponseModeWeighted && weight <= 0 {
		return configError(path, field+".weight", "must be positive for weighted responses")
	}
	return nil
}

func buildDelay(path string, field string, raw *rawDelay) (*mapping.Delay, error) {
	if raw == nil {
		return nil, nil
	}
	switch mapping.DelayType(raw.Type) {
	case mapping.DelayTypeFixed:
		return fixedDelay(path, field, raw.Value)
	case mapping.DelayTypeRandom:
		return randomDelay(path, field, raw.Min, raw.Max)
	default:
		return nil, configError(path, field+".type", "must be fixed or random")
	}
}

func buildDelayDistribution(path string, field string, raw *rawDelayDistribution) (*mapping.Delay, error) {
	switch raw.Type {
	case "uniform":
		return uniformDelayDistribution(path, field, raw.Lower, raw.Upper)
	default:
		return nil, configError(path, field+".type", "must be uniform")
	}
}

func uniformDelayDistribution(path string, field string, lowerValue *int, upperValue *int) (*mapping.Delay, error) {
	if lowerValue == nil {
		return nil, configError(path, field+".lower", "is required")
	}
	if upperValue == nil {
		return nil, configError(path, field+".upper", "is required")
	}
	if *lowerValue < 0 {
		return nil, configError(path, field+".lower", "must be non-negative")
	}
	if *upperValue < 0 {
		return nil, configError(path, field+".upper", "must be non-negative")
	}
	lower := time.Duration(*lowerValue) * time.Millisecond
	upper := time.Duration(*upperValue) * time.Millisecond
	if lower > upper {
		return nil, configError(path, field+".lower", "must be less than or equal to upper")
	}
	return &mapping.Delay{Type: mapping.DelayTypeRandom, Min: lower, Max: upper}, nil
}

func fixedDelay(path string, field string, value string) (*mapping.Delay, error) {
	duration, err := parseNonNegativeDuration(path, field+".value", value)
	if err != nil {
		return nil, err
	}
	return &mapping.Delay{Type: mapping.DelayTypeFixed, Value: duration}, nil
}

func randomDelay(path string, field string, minValue string, maxValue string) (*mapping.Delay, error) {
	minDuration, err := parseNonNegativeDuration(path, field+".min", minValue)
	if err != nil {
		return nil, err
	}
	maxDuration, err := parseNonNegativeDuration(path, field+".max", maxValue)
	if err != nil {
		return nil, err
	}
	if minDuration > maxDuration {
		return nil, configError(path, field+".min", "must be less than or equal to max")
	}
	return &mapping.Delay{Type: mapping.DelayTypeRandom, Min: minDuration, Max: maxDuration}, nil
}

func parseNonNegativeDuration(path string, field string, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, configError(path, field, "must use Go duration syntax")
	}
	if duration < 0 {
		return 0, configError(path, field, "must be non-negative")
	}
	return duration, nil
}

func validateRegex(path string, field string, expression string) error {
	if err := wiremockregex.Validate(expression); err != nil {
		return configError(path, field, "has invalid regex")
	}
	return nil
}
