package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/delay"
	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/domain/matcher"
	"github.com/lHumaNl/gomock/internal/domain/stub"
	"github.com/lHumaNl/gomock/internal/responsetemplate"
)

const (
	healthPath      = "/healthz"
	readyPath       = "/readyz"
	metricsPath     = "/metrics"
	jsonContentType = "application/json"
	defaultVariant  = "default"
	unmatchedStub   = "unmatched"
	unknownStub     = "unknown"
	otherMethod     = "OTHER"
)

type StubMatcher interface {
	Match(request matcher.Request) (stub.Match, bool, error)
}

type RequestMetrics interface {
	RequestStarted()
	RequestFinished(stub string, variant string, method string, status int, matched bool, duration time.Duration)
}

type noopMetrics struct{}

type requestMetricState struct {
	stub    string
	variant string
	method  string
	status  int
	matched bool
}

func (noopMetrics) RequestStarted() {}

func (noopMetrics) RequestFinished(string, string, string, int, bool, time.Duration) {}

func defaultMetrics(metrics RequestMetrics) RequestMetrics {
	if metrics == nil {
		return noopMetrics{}
	}
	return metrics
}

func newRequestMetricState(method string) *requestMetricState {
	return &requestMetricState{stub: unmatchedStub, variant: defaultVariant,
		method: safeHTTPMethod(method), status: http.StatusInternalServerError}
}

func (s *requestMetricState) markMatched(match stub.Match) {
	s.stub = safeLabel(match.MappingID, unknownStub)
	s.variant = safeLabel(match.VariantName, defaultVariant)
	s.status = match.Response.Status
	s.matched = true
}

func (s *requestMetricState) markUnmatched() {
	s.status = http.StatusNotFound
}

func (s *requestMetricState) markError(match stub.Match) {
	if match.MappingID == "" {
		return
	}
	s.stub = safeLabel(match.MappingID, unknownStub)
	s.variant = safeLabel(match.VariantName, defaultVariant)
	s.matched = true
}

type Handler struct {
	matcher     StubMatcher
	ready       func() bool
	logger      *slog.Logger
	delays      *delay.Calculator
	sleepTimer  func(*http.Request, time.Duration) bool
	metrics     RequestMetrics
	metricsHTTP http.Handler
}

func NewHandler(matcher StubMatcher, ready func() bool, logger *slog.Logger) *Handler {
	return NewHandlerWithMetrics(matcher, ready, logger, nil, nil)
}

func NewHandlerWithMetrics(
	matcher StubMatcher,
	ready func() bool,
	logger *slog.Logger,
	metrics RequestMetrics,
	metricsHTTP http.Handler,
) *Handler {
	return &Handler{matcher: matcher, ready: ready, logger: logger,
		delays: delay.NewCalculator(), sleepTimer: sleepTimer,
		metrics: defaultMetrics(metrics), metricsHTTP: metricsHTTP}
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case healthPath:
		h.handleHealth(writer, request)
	case readyPath:
		h.handleReady(writer, request)
	case metricsPath:
		h.handleMetrics(writer, request)
	default:
		h.handleStub(writer, request)
	}
}

func NewMetricsHandler(metricsHTTP http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(metricsPath, metricsHTTP)
	return mux
}

func (h *Handler) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleReady(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	if !h.ready() {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleStub(writer http.ResponseWriter, request *http.Request) {
	state := newRequestMetricState(request.Method)
	h.metrics.RequestStarted()
	started := time.Now()
	defer h.finishRequestMetric(state, started)
	h.dispatchStub(writer, request, state)
}

func (h *Handler) dispatchStub(writer http.ResponseWriter, request *http.Request, state *requestMetricState) {
	model, err := requestModel(request)
	if err != nil {
		h.logger.WarnContext(request.Context(), "request body read failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Failed to read request"})
		return
	}

	matched, found, err := h.matcher.Match(model)
	if err != nil {
		state.markError(matched)
		h.logger.ErrorContext(request.Context(), "matched response failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Failed to serve stub"})
		return
	}
	if !found {
		state.markUnmatched()
		h.logUnmatched(request)
		writeUnmatched(writer, request)
		return
	}
	state.markMatched(matched)
	h.serveMatched(writer, request, model, matched, state)
}

func (h *Handler) handleMetrics(writer http.ResponseWriter, request *http.Request) {
	if h.metricsHTTP == nil {
		h.handleStub(writer, request)
		return
	}
	h.metricsHTTP.ServeHTTP(writer, request)
}

func (h *Handler) finishRequestMetric(state *requestMetricState, started time.Time) {
	h.metrics.RequestFinished(state.stub, state.variant, state.method,
		state.status, state.matched, time.Since(started))
}

func (h *Handler) serveMatched(
	writer http.ResponseWriter,
	request *http.Request,
	model matcher.Request,
	matched stub.Match,
	state *requestMetricState,
) {
	h.logger.DebugContext(request.Context(), "request matched", "stub", matched.MappingID, "variant", matched.VariantName)
	response, err := responsetemplate.RenderResponse(matched.Response, model)
	if err != nil {
		state.status = http.StatusInternalServerError
		h.logger.ErrorContext(request.Context(), "response template failed", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Failed to serve stub"})
		return
	}
	if !h.applyDelay(request, response) {
		return
	}
	writeResponse(writer, response)
}

func (h *Handler) applyDelay(request *http.Request, response mapping.Response) bool {
	duration := h.delays.Duration(response.Delay)
	if duration <= 0 {
		return true
	}
	return h.sleepTimer(request, duration)
}

func (h *Handler) logUnmatched(request *http.Request) {
	h.logger.WarnContext(request.Context(), "no matching stub", "method", request.Method, "path", request.URL.Path)
}

func requestModel(request *http.Request) (matcher.Request, error) {
	body, err := readRequestBody(request)
	if err != nil {
		return matcher.Request{}, err
	}
	return matcher.Request{Method: request.Method, URI: request.URL.RequestURI(), Headers: request.Header, Body: body}, nil
}

func readRequestBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	return io.ReadAll(request.Body)
}

func writeResponse(writer http.ResponseWriter, response mapping.Response) {
	for name, value := range response.Headers {
		writer.Header().Set(name, value)
	}
	body := responseBody(response)
	writer.WriteHeader(response.Status)
	_, _ = writer.Write(body)
}

func responseBody(response mapping.Response) []byte {
	if response.BodyFileName != "" {
		return response.BodyFileContent
	}
	return []byte(response.Body)
}

func allowGet(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet {
		return true
	}
	writer.Header().Set("Allow", http.MethodGet)
	writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	return false
}

func writeJSON(writer http.ResponseWriter, status int, payload map[string]string) {
	writer.Header().Set("Content-Type", jsonContentType)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeUnmatched(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusNotFound, map[string]string{
		"error":  "No matching stub found",
		"method": request.Method,
		"path":   request.URL.Path,
	})
}

func sleepTimer(request *http.Request, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-request.Context().Done():
		return false
	}
}

func safeHTTPMethod(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace:
		return strings.ToUpper(method)
	default:
		return otherMethod
	}
}

func safeLabel(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
