package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/domain/stub"
)

func TestHandlerServesMatchedInlineResponse(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{
		newStub("hello", "/hello", mapping.Response{
			Status:  201,
			Headers: map[string]string{"X-Stub": "hello"},
			Body:    `{"ok":true}`,
		}),
	})

	response := performRequest(handler, http.MethodGet, "/hello")

	assertStatus(t, response, http.StatusCreated)
	assertBody(t, response, `{"ok":true}`)
	if response.Header.Get("X-Stub") != "hello" {
		t.Fatalf("expected X-Stub header, got %q", response.Header.Get("X-Stub"))
	}
}

func TestHandlerServesPreloadedBodyFileContent(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{
		newStub("file", "/file", mapping.Response{
			Status:          http.StatusOK,
			BodyFileName:    "users.json",
			BodyFileContent: []byte(`{"users":[]}`),
		}),
	})

	response := performRequest(handler, http.MethodGet, "/file")

	assertStatus(t, response, http.StatusOK)
	assertBody(t, response, `{"users":[]}`)
}

func TestHandlerReturnsUsefulUnmatched404(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newStub("hello", "/hello", okResponse("ok"))})

	response := performRequest(handler, http.MethodGet, "/missing")

	assertStatus(t, response, http.StatusNotFound)
	assertBodyContainsAll(t, response, "No matching stub found", "/missing")
}

func TestHandlerServesSequentialVariants(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newVariantStub("pages", "/pages", mapping.ResponseModeSequential,
		mapping.Response{Name: "first", Status: http.StatusOK, Body: "page-1"},
		mapping.Response{Name: "second", Status: http.StatusAccepted, Body: "page-2"},
	)})

	first := performRequest(handler, http.MethodGet, "/pages")
	second := performRequest(handler, http.MethodGet, "/pages")
	wrapped := performRequest(handler, http.MethodGet, "/pages")

	assertStatus(t, first, http.StatusOK)
	assertBody(t, first, "page-1")
	assertStatus(t, second, http.StatusAccepted)
	assertBody(t, second, "page-2")
	assertBody(t, wrapped, "page-1")
}

func TestHandlerServesWeightedVariants(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newVariantStub("weighted", "/weighted", mapping.ResponseModeWeighted,
		mapping.Response{Name: "only", Weight: 1, Status: http.StatusCreated, Body: "weighted"},
	)})

	response := performRequest(handler, http.MethodGet, "/weighted")

	assertStatus(t, response, http.StatusCreated)
	assertBody(t, response, "weighted")
}

func TestHandlerServesRandomVariants(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newVariantStub("random", "/random", mapping.ResponseModeRandom,
		mapping.Response{Name: "only", Status: http.StatusOK, Body: "random"},
	)})

	response := performRequest(handler, http.MethodGet, "/random")

	assertStatus(t, response, http.StatusOK)
	assertBody(t, response, "random")
}

func TestHandlerAppliesFixedDelay(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newStub("slow", "/slow", mapping.Response{
		Status: http.StatusOK,
		Body:   "slow",
		Delay:  &mapping.Delay{Type: mapping.DelayTypeFixed, Value: 25 * time.Millisecond},
	})})

	elapsed, response := timedRequest(handler, http.MethodGet, "/slow")

	assertStatus(t, response, http.StatusOK)
	assertBody(t, response, "slow")
	assertElapsedBetween(t, elapsed, 20*time.Millisecond, 500*time.Millisecond)
}

func TestHandlerAppliesRandomDelay(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newStub("slow-random", "/slow-random", mapping.Response{
		Status: http.StatusOK,
		Body:   "slow",
		Delay:  &mapping.Delay{Type: mapping.DelayTypeRandom, Min: 20 * time.Millisecond, Max: 40 * time.Millisecond},
	})})

	elapsed, response := timedRequest(handler, http.MethodGet, "/slow-random")

	assertStatus(t, response, http.StatusOK)
	assertElapsedBetween(t, elapsed, 15*time.Millisecond, 500*time.Millisecond)
}

func TestHandlerHealthAndReadyBypassStubs(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{
		newStub("health-stub", healthPath, mapping.Response{Status: http.StatusTeapot, Body: "stubbed"}),
		newStub("ready-stub", readyPath, mapping.Response{Status: http.StatusTeapot, Body: "stubbed"}),
	})

	health := performRequest(handler, http.MethodGet, healthPath)
	ready := performRequest(handler, http.MethodGet, readyPath)

	assertStatus(t, health, http.StatusOK)
	assertBodyContains(t, health, `"status":"ok"`)
	assertStatus(t, ready, http.StatusOK)
	assertBodyContains(t, ready, `"status":"ok"`)
}

func newTestHandler(mappings []mapping.Mapping) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(stub.NewService(mappings), func() bool { return true }, logger)
}

func newStub(id string, path string, response mapping.Response) mapping.Mapping {
	return mapping.Mapping{ID: id, Request: mapping.Request{Method: http.MethodGet,
		URLKind: mapping.URLMatchKindURLPath, URLValue: path}, Response: &response}
}

func newVariantStub(id string, path string, mode mapping.ResponseMode, variants ...mapping.Response) mapping.Mapping {
	return mapping.Mapping{ID: id, Request: mapping.Request{Method: http.MethodGet,
		URLKind: mapping.URLMatchKindURLPath, URLValue: path}, Responses: &mapping.ResponseSet{Mode: mode, Variants: variants}}
}

func okResponse(body string) mapping.Response {
	return mapping.Response{Status: http.StatusOK, Body: body}
}

func performRequest(handler http.Handler, method string, path string) *http.Response {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder.Result()
}

func timedRequest(handler http.Handler, method string, path string) (time.Duration, *http.Response) {
	start := time.Now()
	response := performRequest(handler, method, path)
	return time.Since(start), response
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		t.Fatalf("expected status %d, got %d", want, response.StatusCode)
	}
}

func assertBody(t *testing.T, response *http.Response, want string) {
	t.Helper()
	body := readBody(t, response)
	if body != want {
		t.Fatalf("expected body %q, got %q", want, body)
	}
}

func assertBodyContains(t *testing.T, response *http.Response, want string) {
	t.Helper()
	assertBodyContainsAll(t, response, want)
}

func assertBodyContainsAll(t *testing.T, response *http.Response, wants ...string) {
	t.Helper()
	body := readBody(t, response)
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body %q to contain %q", body, want)
		}
	}
}

func assertElapsedBetween(t *testing.T, elapsed time.Duration, minimum time.Duration, maximum time.Duration) {
	t.Helper()
	if elapsed < minimum || elapsed > maximum {
		t.Fatalf("expected elapsed between %s and %s, got %s", minimum, maximum, elapsed)
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer closeBody(response.Body)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(body)
}

func closeBody(body io.Closer) {
	_ = body.Close()
}
