package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lHumaNl/gomock/internal/domain/matcher"
)

const (
	VerboseOff     = "off"
	VerboseSummary = "summary"
	VerboseFull    = "full"

	defaultVerboseBodyLimit    = 4096
	defaultVerbosePreviewLimit = 160
	redactedValue              = "[REDACTED]"
)

type VerboseConfig struct {
	Mode         string
	BodyLimit    int
	PreviewLimit int
	Redact       bool
}

type verboseLogger struct {
	config VerboseConfig
}

type requestLogState struct {
	request  matcher.Request
	response *responseLogRecorder
}

type responseLogRecorder struct {
	http.ResponseWriter
	status    int
	body      bytes.Buffer
	seenBytes int
	limit     int
	truncated bool
}

func newVerboseLogger(config VerboseConfig) verboseLogger {
	return verboseLogger{config: normalizeVerboseConfig(config)}
}

func normalizeVerboseConfig(config VerboseConfig) VerboseConfig {
	if config.Mode == "" {
		config.Mode = VerboseOff
	}
	if config.BodyLimit <= 0 {
		config.BodyLimit = defaultVerboseBodyLimit
	}
	if config.PreviewLimit < 1 {
		config.PreviewLimit = defaultVerbosePreviewLimit
	}
	return config
}

func (v verboseLogger) newState(request *http.Request) *requestLogState {
	return &requestLogState{request: requestModelFromHTTP(request, nil)}
}

func (v verboseLogger) wrap(w http.ResponseWriter, state *requestLogState, path string) http.ResponseWriter {
	if !v.fullEnabled(path) {
		return w
	}
	state.response = newResponseLogRecorder(w, v.config.BodyLimit)
	return state.response
}

func (v verboseLogger) log(
	logger *slog.Logger,
	request *http.Request,
	metrics *requestMetricState,
	state *requestLogState,
	duration time.Duration,
) {
	if !v.enabled(request.URL.Path) {
		return
	}
	attrs := v.summaryAttrs(request, metrics, duration)
	if v.config.Mode == VerboseFull {
		attrs = append(attrs, v.fullAttrs(metrics, state)...)
	}
	logger.InfoContext(request.Context(), "traffic", attrs...)
}

func (v verboseLogger) enabled(path string) bool {
	return v.config.Mode != VerboseOff && v.allowedPath(path)
}

func (v verboseLogger) fullEnabled(path string) bool {
	return v.config.Mode == VerboseFull && v.allowedPath(path)
}

func (v verboseLogger) allowedPath(path string) bool {
	return !isProbePath(path)
}

func (v verboseLogger) summaryAttrs(request *http.Request, state *requestMetricState, duration time.Duration) []any {
	return []any{
		"request", v.summaryRequest(request),
		"matched", state.matched,
		"stub", state.stub,
		"variant", state.variant,
		"status", state.status,
		"duration_ms", duration.Milliseconds(),
	}
}

func (v verboseLogger) fullAttrs(metrics *requestMetricState, state *requestLogState) []any {
	return []any{
		"request_detail", v.requestDetails(state.request),
		"response_detail", v.responseDetails(metrics, state.response),
	}
}

func (v verboseLogger) summaryRequest(request *http.Request) string {
	value := request.Method + " " + request.URL.RequestURI()
	return truncateString(v.redactURI(value), v.config.PreviewLimit)
}

func (v verboseLogger) requestDetails(request matcher.Request) map[string]any {
	body, truncated := v.bodyValue(request.Body)
	return map[string]any{"method": request.Method, "uri": v.redactURI(request.URI),
		"headers": v.headerValue(http.Header(request.Headers)), "body": body, "body_truncated": truncated}
}

func (v verboseLogger) responseDetails(state *requestMetricState, recorder *responseLogRecorder) map[string]any {
	if recorder == nil {
		return map[string]any{"status": state.status}
	}
	body, truncated := v.bodyValue(recorder.body.Bytes())
	return map[string]any{"status": responseStatus(state, recorder),
		"headers": v.headerValue(recorder.Header()), "body": body, "body_truncated": truncated || recorder.truncated}
}

func (v verboseLogger) bodyValue(body []byte) (string, bool) {
	value := string(body)
	if v.config.Redact && containsSensitiveName(value) {
		value = redactedValue
	}
	return truncateString(value, v.config.BodyLimit), len(value) > v.config.BodyLimit
}

func (v verboseLogger) headerValue(headers http.Header) http.Header {
	copied := make(http.Header, len(headers))
	for name, values := range headers {
		copied[name] = v.headerValues(name, values)
	}
	return copied
}

func (v verboseLogger) headerValues(name string, values []string) []string {
	if v.config.Redact && sensitiveName(name) {
		return repeatedRedactedValues(values)
	}
	return append([]string(nil), values...)
}

func (v verboseLogger) redactURI(value string) string {
	if !v.config.Redact {
		return value
	}
	method, rawURI, ok := strings.Cut(value, " ")
	if !ok {
		return redactQueryValues(value)
	}
	return method + " " + redactQueryValues(rawURI)
}

func newResponseLogRecorder(w http.ResponseWriter, limit int) *responseLogRecorder {
	return &responseLogRecorder{ResponseWriter: w, limit: limit}
}

func (r *responseLogRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseLogRecorder) Write(payload []byte) (int, error) {
	r.capture(payload)
	return r.ResponseWriter.Write(payload)
}

func (r *responseLogRecorder) capture(payload []byte) {
	r.seenBytes += len(payload)
	available := r.limit - r.body.Len()
	if available > 0 {
		r.body.Write(payload[:min(len(payload), available)])
	}
	r.truncated = r.seenBytes > r.limit
}

func requestModelFromHTTP(request *http.Request, body []byte) matcher.Request {
	return matcher.Request{Method: request.Method, URI: request.URL.RequestURI(), Headers: request.Header, Body: body}
}

func responseStatus(state *requestMetricState, recorder *responseLogRecorder) int {
	if recorder.status != 0 {
		return recorder.status
	}
	return state.status
}

func truncateString(value string, limit int) string {
	if limit < 1 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func repeatedRedactedValues(values []string) []string {
	redacted := make([]string, len(values))
	for index := range values {
		redacted[index] = redactedValue
	}
	return redacted
}

func sensitiveName(name string) bool {
	lower := strings.ToLower(name)
	return lower == "authorization" || lower == "cookie" || lower == "proxy-authorization" ||
		lower == "set-cookie" || containsSensitiveName(lower)
}

func containsSensitiveName(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "token") || strings.Contains(lower, "password")
}

func redactQueryValues(rawURI string) string {
	parsed, err := url.ParseRequestURI(rawURI)
	if err != nil {
		return rawURI
	}
	query := parsed.Query()
	for key := range query {
		if sensitiveName(key) {
			query[key] = repeatedRedactedValues(query[key])
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.RequestURI()
}

func isProbePath(path string) bool {
	return path == healthPath || path == readyPath || path == metricsPath
}
