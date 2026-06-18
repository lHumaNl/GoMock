package selector

import (
	"net/http"
	"testing"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func BenchmarkSequentialSelector(b *testing.B) {
	benchmarkSelector(b, NewSequential(benchmarkVariants()))
}

func BenchmarkRandomSelector(b *testing.B) {
	benchmarkSelector(b, NewRandom(benchmarkVariants()))
}

func BenchmarkWeightedSelector(b *testing.B) {
	benchmarkSelector(b, NewWeighted(benchmarkWeightedVariants()))
}

func benchmarkSelector(b *testing.B, selector Selector) {
	b.Helper()
	total := 0
	b.ReportAllocs()
	for range b.N {
		total += selector.Select().Status
	}
	if total == 0 {
		b.Fatal("selector returned no statuses")
	}
}

func benchmarkVariants() []mapping.Response {
	return []mapping.Response{
		{Name: "first", Status: http.StatusOK},
		{Name: "second", Status: http.StatusAccepted},
		{Name: "third", Status: http.StatusNoContent},
	}
}

func benchmarkWeightedVariants() []mapping.Response {
	return []mapping.Response{
		{Name: "ok", Weight: 90, Status: http.StatusOK},
		{Name: "error", Weight: 10, Status: http.StatusInternalServerError},
	}
}
