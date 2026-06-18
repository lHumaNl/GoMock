package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/domain/stub"
	"github.com/lHumaNl/gomock/internal/observability"
)

func TestHandlerMetricsRecordsMatchedAndUnmatchedRequests(t *testing.T) {
	handler := newMetricsTestHandler(t, []mapping.Mapping{
		newStub("hello", "/hello", mapping.Response{Name: "ignored", Status: http.StatusCreated}),
	})

	performRequest(handler, http.MethodGet, "/hello")
	performRequest(handler, http.MethodGet, "/missing?user=123")
	metrics := scrapeMetrics(t, handler)

	assertMetricLine(t, metrics, "gomock_requests_total", []string{
		`stub="hello"`, `variant="default"`, `method="GET"`, `status="201"`, `matched="true"`,
	}, 1)
	assertMetricLine(t, metrics, "gomock_requests_total", []string{
		`stub="unmatched"`, `variant="default"`, `method="GET"`, `status="404"`, `matched="false"`,
	}, 1)
}

func TestHandlerMetricsExposeRuntimeGaugesAndBuildInfo(t *testing.T) {
	handler := newMetricsTestHandler(t, []mapping.Mapping{
		newStub("one", "/one", okResponse("one")),
		newStub("two", "/two", okResponse("two")),
	})

	metrics := scrapeMetrics(t, handler)

	assertMetricLine(t, metrics, "gomock_mappings_loaded", nil, 2)
	assertMetricLabels(t, metrics, "gomock_build_info", []string{
		`version="test-version"`, `commit="test-commit"`, `go_version="go`,
	})
}

func TestHandlerInFlightGaugeTracksRunningRequests(t *testing.T) {
	handler := newMetricsTestHandler(t, []mapping.Mapping{newStub("slow", "/slow", mapping.Response{
		Status: http.StatusOK,
		Delay:  &mapping.Delay{Type: mapping.DelayTypeFixed, Value: time.Minute},
	})})
	started := make(chan struct{})
	release := make(chan struct{})
	handler.sleepTimer = blockingTimer(started, release)
	done := make(chan struct{})

	go func() {
		performRequest(handler, http.MethodGet, "/slow")
		close(done)
	}()
	<-started
	assertMetricLine(t, scrapeMetrics(t, handler), "gomock_inflight_requests", nil, 1)
	close(release)
	<-done
	assertMetricLine(t, scrapeMetrics(t, handler), "gomock_inflight_requests", nil, 0)
}

func TestHandlerDurationHistogramIncludesConfiguredDelay(t *testing.T) {
	handler := newMetricsTestHandler(t, []mapping.Mapping{newStub("slow", "/slow", mapping.Response{
		Status: http.StatusOK,
		Delay:  &mapping.Delay{Type: mapping.DelayTypeFixed, Value: 30 * time.Millisecond},
	})})

	performRequest(handler, http.MethodGet, "/slow")
	metrics := scrapeMetrics(t, handler)

	assertMetricAtLeast(t, metrics, "gomock_request_duration_seconds_sum", []string{
		`stub="slow"`, `variant="default"`, `method="GET"`, `status="200"`, `matched="true"`,
	}, 0.02)
}

func TestHandlerMetricsAvoidHighCardinalityRequestData(t *testing.T) {
	handler := newMetricsTestHandler(t, []mapping.Mapping{newStub("hello", "/hello", okResponse("ok"))})
	request := httptest.NewRequest(http.MethodPost, "/secret-path?token=secret-token", strings.NewReader("secret-body"))
	request.Header.Set("X-User-ID", "dynamic-user-123")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	metrics := scrapeMetrics(t, handler)

	assertNotContains(t, metrics, "secret-path", "secret-token", "secret-body", "dynamic-user-123")
}

func newMetricsTestHandler(t *testing.T, mappings []mapping.Mapping) *Handler {
	t.Helper()
	metrics, err := observability.NewMetrics(nil, observability.BuildInfo{
		Version: "test-version",
		Commit:  "test-commit",
	})
	if err != nil {
		t.Fatalf("new metrics: %v", err)
	}
	metrics.SetMappingsLoaded(len(mappings))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandlerWithMetrics(stub.NewService(mappings), func() bool { return true }, logger, metrics, metrics.Handler())
}

func scrapeMetrics(t *testing.T, handler http.Handler) string {
	t.Helper()
	response := performRequest(handler, http.MethodGet, metricsPath)
	defer closeBody(response.Body)
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	return string(content)
}

func blockingTimer(started chan<- struct{}, release <-chan struct{}) func(*http.Request, time.Duration) bool {
	return func(*http.Request, time.Duration) bool {
		close(started)
		<-release
		return true
	}
}

func assertMetricLine(t *testing.T, metrics string, name string, labels []string, want float64) {
	t.Helper()
	line := findMetricLine(t, metrics, name, labels)
	if got := metricValue(t, line); got != want {
		t.Fatalf("expected %s to be %v, got line %q", name, want, line)
	}
}

func assertMetricAtLeast(t *testing.T, metrics string, name string, labels []string, minimum float64) {
	t.Helper()
	line := findMetricLine(t, metrics, name, labels)
	if got := metricValue(t, line); got < minimum {
		t.Fatalf("expected %s to be at least %v, got line %q", name, minimum, line)
	}
}

func assertMetricLabels(t *testing.T, metrics string, name string, labels []string) {
	t.Helper()
	_ = findMetricLine(t, metrics, name, labels)
}

func findMetricLine(t *testing.T, metrics string, name string, labels []string) string {
	t.Helper()
	for _, line := range strings.Split(metrics, "\n") {
		if isMetricLine(line, name, labels) {
			return line
		}
	}
	t.Fatalf("metric %s with labels %v not found in:\n%s", name, labels, metrics)
	return ""
}

func isMetricLine(line string, name string, labels []string) bool {
	if !strings.HasPrefix(line, name) {
		return false
	}
	for _, label := range labels {
		if !strings.Contains(line, label) {
			return false
		}
	}
	return true
}

func metricValue(t *testing.T, line string) float64 {
	t.Helper()
	parts := strings.Fields(line)
	if len(parts) == 0 {
		t.Fatalf("metric line has no value: %q", line)
	}
	value, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil {
		t.Fatalf("parse metric value from %q: %v", line, err)
	}
	return value
}

func assertNotContains(t *testing.T, content string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(content, value) {
			t.Fatalf("expected metrics not to contain %q", value)
		}
	}
}
