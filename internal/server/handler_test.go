package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
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

func TestHandlerServesBase64BodyBytes(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{
		newStub("binary", "/binary", mapping.Response{
			Status:       http.StatusOK,
			BodyBytes:    []byte{0x00, 0x01, 0x02},
			BodyBytesSet: true,
		}),
	})

	response := performRequest(handler, http.MethodGet, "/binary")

	assertStatus(t, response, http.StatusOK)
	assertBodyBytes(t, response, []byte{0x00, 0x01, 0x02})
}

func TestHandlerMatchesCookiesAndBasicAuth(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{secureCookieStub()})

	matched := performSecureRequest(handler, "abc123", "api", "secret")
	unmatched := performSecureRequest(handler, "abc123", "api", "wrong")

	assertStatus(t, matched, http.StatusOK)
	assertBody(t, matched, "secure")
	assertStatus(t, unmatched, http.StatusNotFound)
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

func TestHandlerRendersResponseTemplateInlineBody(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newPostStub("templated", "/templated", mapping.Response{
		Status:       http.StatusOK,
		Headers:      map[string]string{"X-Echo": "{{jsonPath originalRequest.body '$.path'}}"},
		Body:         userStyleTemplate(),
		Transformers: []string{mapping.TransformerResponseTemplate},
	})})

	response := performRequestWithBody(handler, http.MethodPost, "/templated", requestTemplateBody())

	assertStatus(t, response, http.StatusOK)
	assertTemplatedJSON(t, response)
	if response.Header.Get("X-Echo") != "alpha" {
		t.Fatalf("expected templated X-Echo header, got %q", response.Header.Get("X-Echo"))
	}
}

func TestHandlerRendersResponseTemplateBodyFile(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newPostStub("file-template", "/file-template", mapping.Response{
		Status:          http.StatusOK,
		BodyFileName:    "template.json",
		BodyFileContent: []byte(`{"echo":"{{jsonPath originalRequest.body '$.path'}}"}`),
		Transformers:    []string{mapping.TransformerResponseTemplate},
	})})

	response := performRequestWithBody(handler, http.MethodPost, "/file-template", requestTemplateBody())

	assertStatus(t, response, http.StatusOK)
	assertBody(t, response, `{"echo":"alpha"}`)
}

func TestHandlerDoesNotRenderTemplateWithoutTransformer(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newPostStub("static", "/static", mapping.Response{
		Status: http.StatusOK,
		Body:   `{{jsonPath originalRequest.body '$.path'}}`,
	})})

	response := performRequestWithBody(handler, http.MethodPost, "/static", requestTemplateBody())

	assertStatus(t, response, http.StatusOK)
	assertBody(t, response, `{{jsonPath originalRequest.body '$.path'}}`)
}

func TestHandlerReturnsErrorForInvalidResponseTemplate(t *testing.T) {
	handler := newTestHandler([]mapping.Mapping{newPostStub("bad-template", "/bad-template", mapping.Response{
		Status:       http.StatusOK,
		Body:         `{{jsonPath originalRequest.body '$['}}`,
		Transformers: []string{mapping.TransformerResponseTemplate},
	})})

	response := performRequestWithBody(handler, http.MethodPost, "/bad-template", requestTemplateBody())

	assertStatus(t, response, http.StatusInternalServerError)
	assertBodyContains(t, response, "Failed to serve stub")
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

func newPostStub(id string, path string, response mapping.Response) mapping.Mapping {
	return mapping.Mapping{ID: id, Request: mapping.Request{Method: http.MethodPost,
		URLKind: mapping.URLMatchKindURLPath, URLValue: path}, Response: &response}
}

func newVariantStub(id string, path string, mode mapping.ResponseMode, variants ...mapping.Response) mapping.Mapping {
	return mapping.Mapping{ID: id, Request: mapping.Request{Method: http.MethodGet,
		URLKind: mapping.URLMatchKindURLPath, URLValue: path}, Responses: &mapping.ResponseSet{Mode: mode, Variants: variants}}
}

func secureCookieStub() mapping.Mapping {
	return mapping.Mapping{ID: "secure", Request: mapping.Request{Method: http.MethodGet,
		URLKind: mapping.URLMatchKindURLPath, URLValue: "/secure",
		Cookies:   map[string]mapping.Matcher{"session": {Operator: mapping.OperatorContains, Value: "abc"}},
		BasicAuth: &mapping.BasicAuth{Username: "api", Password: "secret"}}, Response: &mapping.Response{Status: http.StatusOK, Body: "secure"}}
}

func okResponse(body string) mapping.Response {
	return mapping.Response{Status: http.StatusOK, Body: body}
}

func performRequest(handler http.Handler, method string, path string) *http.Response {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder.Result()
}

func performRequestWithBody(handler http.Handler, method string, path string, body string) *http.Response {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

func performSecureRequest(handler http.Handler, session string, username string, password string) *http.Response {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/secure", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: session})
	request.SetBasicAuth(username, password)
	handler.ServeHTTP(recorder, request)
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

func assertBodyBytes(t *testing.T, response *http.Response, want []byte) {
	t.Helper()
	body := readBodyBytes(t, response)
	if string(body) != string(want) {
		t.Fatalf("expected body bytes %v, got %v", want, body)
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

func assertTemplatedJSON(t *testing.T, response *http.Response) {
	t.Helper()
	var payload templatedPayload
	if err := json.Unmarshal([]byte(readBody(t, response)), &payload); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	assertTemplatedPayload(t, payload)
}

func assertTemplatedPayload(t *testing.T, payload templatedPayload) {
	t.Helper()
	if payload.Path != "alpha" || len(payload.Items) != 2 {
		t.Fatalf("unexpected templated payload: %#v", payload)
	}
	if payload.Items[0].Field != "one" || payload.Items[1].Field != "two" {
		t.Fatalf("unexpected templated items: %#v", payload.Items)
	}
	if payload.Int < 1 || payload.Int > 100 || !regexp.MustCompile(`^\d{6}$`).MatchString(payload.Code) {
		t.Fatalf("unexpected random values: %#v", payload)
	}
}

type templatedPayload struct {
	Path  string          `json:"path"`
	Items []templatedItem `json:"items"`
	Int   int             `json:"int"`
	Code  string          `json:"code"`
}

type templatedItem struct {
	Field string `json:"field"`
}

func requestTemplateBody() string {
	return `{"path":"alpha","array":[{"field":"one"},{"field":"two"}]}`
}

func userStyleTemplate() string {
	return `{"path":"{{jsonPath originalRequest.body '$.path'}}","items":[` +
		`{{#each (jsonPath originalRequest.body '$.array') as |item|}}` +
		`{"field":"{{jsonPath item '$.field'}}"}{{#unless @last}},{{/unless}}` +
		`{{/each}}],"int":{{randomInt lower=1 upper=100}},` +
		`"code":"{{randomValue length=6 type='NUMERIC'}}"}`
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	return string(readBodyBytes(t, response))
}

func readBodyBytes(t *testing.T, response *http.Response) []byte {
	t.Helper()
	defer closeBody(response.Body)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return body
}

func closeBody(body io.Closer) {
	_ = body.Close()
}
