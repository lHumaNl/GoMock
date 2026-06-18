package matcher

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func BenchmarkSelectMappings(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("mappings_%d", size), func(b *testing.B) {
			items := benchmarkMappings(size)
			request := Request{Method: http.MethodGet, URI: fmt.Sprintf("/resource/%d", size-1)}
			b.ReportAllocs()
			for range b.N {
				selection := Select(items, request)
				if !selection.Found() {
					b.Fatal("expected selected mapping")
				}
			}
		})
	}
}

func BenchmarkMatchHeaderQueryAndBody(b *testing.B) {
	item := mapping.Mapping{ID: "complex", Request: benchmarkComplexRequest()}
	request := Request{Method: http.MethodPost, URI: "/orders?search=active-user", Headers: benchmarkHeaders(), Body: benchmarkBody()}
	b.ReportAllocs()
	for range b.N {
		result := Match(item, request)
		if !result.Matched {
			b.Fatalf("expected match, got %s", result.Reason)
		}
	}
}

func benchmarkMappings(count int) []mapping.Mapping {
	items := make([]mapping.Mapping, 0, count)
	for index := range count {
		items = append(items, benchmarkMapping(index))
	}
	return items
}

func benchmarkMapping(index int) mapping.Mapping {
	return mapping.Mapping{ID: fmt.Sprintf("resource-%d", index), Request: mapping.Request{
		Method: http.MethodGet, URLKind: mapping.URLMatchKindURLPath, URLValue: fmt.Sprintf("/resource/%d", index),
	}, Response: &mapping.Response{Status: http.StatusOK, Body: "ok"}}
}

func benchmarkComplexRequest() mapping.Request {
	return mapping.Request{Method: http.MethodPost, URLKind: mapping.URLMatchKindURLPath, URLValue: "/orders",
		Headers: benchmarkHeaderMatchers(), QueryParameters: benchmarkQueryMatchers(), BodyPatterns: benchmarkBodyMatchers()}
}

func benchmarkHeaderMatchers() map[string]mapping.Matcher {
	return map[string]mapping.Matcher{"X-Request-ID": {Operator: mapping.OperatorMatches, Value: "^[a-z0-9-]+$"}}
}

func benchmarkQueryMatchers() map[string]mapping.Matcher {
	return map[string]mapping.Matcher{"search": {Operator: mapping.OperatorContains, Value: "active"}}
}

func benchmarkBodyMatchers() []mapping.Matcher {
	return []mapping.Matcher{{Operator: mapping.OperatorContains, Value: "user"}, {Operator: mapping.OperatorMatchesJSONPath, Value: "$.user.id"}}
}

func benchmarkHeaders() map[string][]string {
	return map[string][]string{"X-Request-ID": {"abc-123"}}
}

func benchmarkBody() []byte {
	return []byte(`{"user":{"id":42,"active":true},"items":[{"sku":"A"}]}`)
}
