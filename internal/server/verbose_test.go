package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/domain/stub"
)

func TestVerboseSummaryLogsMatchedAndUnmatchedRequests(t *testing.T) {
	handler, logs := newVerboseTestHandler([]mapping.Mapping{
		newStub("hello", "/hello", okResponse("ok")),
	}, VerboseConfig{Mode: VerboseSummary})

	closeBody(performRequest(handler, http.MethodGet, "/hello").Body)
	closeBody(performRequest(handler, http.MethodGet, "/missing").Body)
	closeBody(performRequest(handler, http.MethodGet, healthPath).Body)
	entries := readTrafficLogs(t, logs)

	if len(entries) != 2 {
		t.Fatalf("expected two traffic logs, got %d: %s", len(entries), logs.String())
	}
	assertLogFields(t, entries[0], "GET /hello", true, "hello", http.StatusOK)
	assertLogFields(t, entries[1], "GET /missing", false, unmatchedStub, http.StatusNotFound)
}

func TestVerboseFullShowsSensitiveValuesWhenRedactionDisabled(t *testing.T) {
	handler, logs := newVerboseTestHandler([]mapping.Mapping{newPostStub("secret", "/secret", mapping.Response{
		Status: http.StatusAccepted, Headers: map[string]string{"Set-Cookie": "server=session"}, Body: "token=server-token",
	})}, VerboseConfig{Mode: VerboseFull})

	request := newSecretRequest()
	closeBody(serveRequest(handler, request).Body)
	content := logs.String()

	assertStringContains(t, content, "Bearer request-secret")
	assertStringContains(t, content, "client=session")
	assertStringContains(t, content, "request-token")
	assertStringContains(t, content, "server-token")
}

func TestVerboseFullRedactsSensitiveValuesWhenEnabled(t *testing.T) {
	handler, logs := newVerboseTestHandler([]mapping.Mapping{newPostStub("secret", "/secret", mapping.Response{
		Status: http.StatusOK, Headers: map[string]string{"Set-Cookie": "server=session"}, Body: "password=server-password",
	})}, VerboseConfig{Mode: VerboseFull, Redact: true})

	request := newSecretRequest()
	closeBody(serveRequest(handler, request).Body)
	content := logs.String()

	assertStringNotContains(t, content, "Bearer request-secret")
	assertStringNotContains(t, content, "client=session")
	assertStringNotContains(t, content, "query-token")
	assertStringNotContains(t, content, "request-token")
	assertStringNotContains(t, content, "server-password")
	assertStringContains(t, content, redactedValue)
}

func TestVerboseFullTruncatesBodies(t *testing.T) {
	handler, logs := newVerboseTestHandler([]mapping.Mapping{newPostStub("large", "/large", mapping.Response{
		Status: http.StatusOK, Body: "0123456789",
	})}, VerboseConfig{Mode: VerboseFull, BodyLimit: 5})

	request := httptest.NewRequest(http.MethodPost, "/large", strings.NewReader("abcdefghijklmnopqrstuvwxyz"))
	closeBody(serveRequest(handler, request).Body)
	content := logs.String()

	assertStringContains(t, content, "abcde")
	assertStringContains(t, content, "01234")
	assertStringNotContains(t, content, "abcdef")
	assertStringNotContains(t, content, "012345")
	assertStringContains(t, content, `"body_truncated":true`)
}

func newVerboseTestHandler(mappings []mapping.Mapping, config VerboseConfig) (http.Handler, *bytes.Buffer) {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	handler := NewHandlerWithOptions(stub.NewService(mappings), func() bool { return true }, logger, nil, nil, config)
	return handler, logs
}

func newSecretRequest() *http.Request {
	body := `{"token":"request-token","password":"request-password"}`
	request := httptest.NewRequest(http.MethodPost, "/secret?token=query-token", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer request-secret")
	request.Header.Set("Cookie", "client=session")
	return request
}

func serveRequest(handler http.Handler, request *http.Request) *http.Response {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

func readTrafficLogs(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	entries := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if line == "" || !strings.Contains(line, `"msg":"traffic"`) {
			continue
		}
		entries = append(entries, decodeLogLine(t, line))
	}
	return entries
}

func decodeLogLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("decode log line %q: %v", line, err)
	}
	return entry
}

func assertLogFields(t *testing.T, entry map[string]any, request string, matched bool, stub string, status int) {
	t.Helper()
	if entry["request"] != request || entry["matched"] != matched || entry["stub"] != stub {
		t.Fatalf("unexpected traffic log fields: %#v", entry)
	}
	if entry["status"] != float64(status) {
		t.Fatalf("expected status %d, got %#v", status, entry["status"])
	}
}

func assertStringContains(t *testing.T, content string, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("expected logs to contain %q in %s", want, content)
	}
}

func assertStringNotContains(t *testing.T, content string, want string) {
	t.Helper()
	if strings.Contains(content, want) {
		t.Fatalf("expected logs not to contain %q in %s", want, content)
	}
}
