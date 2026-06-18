package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/domain/stub"
)

func BenchmarkHandlerInlineResponse(b *testing.B) {
	handler := benchmarkHandler(mapping.Response{Status: http.StatusOK, Body: `{"ok":true}`})
	benchmarkHandlerResponse(b, handler)
}

func BenchmarkHandlerBodyFileResponse(b *testing.B) {
	response := mapping.Response{Status: http.StatusOK, BodyFileName: "users.json", BodyFileContent: []byte(`{"users":[]}`)}
	handler := benchmarkHandler(response)
	benchmarkHandlerResponse(b, handler)
}

func benchmarkHandler(response mapping.Response) http.Handler {
	item := mapping.Mapping{ID: "bench", Request: mapping.Request{
		Method: http.MethodGet, URLKind: mapping.URLMatchKindURLPath, URLValue: "/bench",
	}, Response: &response}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(stub.NewService([]mapping.Mapping{item}), func() bool { return true }, logger)
}

func benchmarkHandlerResponse(b *testing.B, handler http.Handler) {
	b.Helper()
	b.ReportAllocs()
	for range b.N {
		request := httptest.NewRequest(http.MethodGet, "/bench", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		response := recorder.Result()
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			b.Fatalf("expected status 200, got %d", response.StatusCode)
		}
	}
}
