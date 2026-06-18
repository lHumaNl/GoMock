package perf

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	perfOptInEnv              = "GOMOCK_PERF"
	defaultPerfMappings       = 100
	defaultPerfConcurrency    = 16
	defaultPerfDuration       = 3 * time.Second
	defaultPerfRequestTimeout = 2 * time.Second
	startupTimeout            = 5 * time.Second
)

var latencyBuckets = []time.Duration{
	100 * time.Microsecond,
	250 * time.Microsecond,
	500 * time.Microsecond,
	time.Millisecond,
	2 * time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	20 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2 * time.Second,
	5 * time.Second,
}

type perfConfig struct {
	mappings       int
	concurrency    int
	duration       time.Duration
	requestTimeout time.Duration
	cpuProfile     string
	memProfile     string
}

type loadResult struct {
	duration time.Duration
	status   int
	err      error
	canceled bool
}

type loadSummary struct {
	requests     int64
	errors       int64
	nonOK        int64
	elapsed      time.Duration
	statusCounts map[int]int64
	latency      latencyHistogram
}

type latencyHistogram struct {
	buckets []int64
	count   int64
	sum     time.Duration
	min     time.Duration
	max     time.Duration
}

type runtimeSnapshot struct {
	heapAlloc   uint64
	heapInuse   uint64
	heapObjects uint64
	numGC       uint32
	goroutines  int
}

func TestPerformanceSmoke(t *testing.T) {
	if os.Getenv(perfOptInEnv) != "1" {
		t.Skipf("set %s=1 to run the opt-in performance smoke test", perfOptInEnv)
	}
	cfg := perfConfigFromEnv(t)
	stopCPUProfile := startCPUProfile(t, cfg.cpuProfile)
	defer stopCPUProfile()

	baseURL := startGoMock(t, generatedRoot(t, cfg.mappings))
	runtime.GC()
	before := captureRuntime()
	summary := runLoad(t, cfg, baseURL)
	runtime.GC()
	after := captureRuntime()
	writeMemoryProfile(t, cfg.memProfile)

	logLoadSummary(t, cfg, summary)
	logRuntimeSummary(t, before, after)
	assertLoadSucceeded(t, summary)
}

func BenchmarkHTTPServerLoad(b *testing.B) {
	cfg := perfConfigFromEnv(b)
	baseURL := startGoMock(b, generatedRoot(b, cfg.mappings))
	client := newHTTPClient(cfg)
	b.Cleanup(client.CloseIdleConnections)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		requestIndex := 0
		for pb.Next() {
			runBenchmarkRequest(b, client, baseURL, cfg.mappings, requestIndex)
			requestIndex++
		}
	})
}

func perfConfigFromEnv(tb testing.TB) perfConfig {
	tb.Helper()
	return perfConfig{
		mappings:       intEnv(tb, "GOMOCK_PERF_MAPPINGS", defaultPerfMappings),
		concurrency:    intEnv(tb, "GOMOCK_PERF_CONCURRENCY", defaultPerfConcurrency),
		duration:       durationEnv(tb, "GOMOCK_PERF_DURATION", defaultPerfDuration),
		requestTimeout: durationEnv(tb, "GOMOCK_PERF_REQUEST_TIMEOUT", defaultPerfRequestTimeout),
		cpuProfile:     os.Getenv("GOMOCK_PERF_CPU_PROFILE"),
		memProfile:     os.Getenv("GOMOCK_PERF_MEM_PROFILE"),
	}
}

func intEnv(tb testing.TB, name string, fallback int) int {
	tb.Helper()
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		tb.Fatalf("%s must be a positive integer, got %q", name, raw)
	}
	return value
}

func durationEnv(tb testing.TB, name string, fallback time.Duration) time.Duration {
	tb.Helper()
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		tb.Fatalf("%s must be a positive Go duration, got %q", name, raw)
	}
	return value
}

func generatedRoot(tb testing.TB, mappings int) string {
	tb.Helper()
	root := tb.TempDir()
	mappingsDir := filepath.Join(root, "mappings")
	filesDir := filepath.Join(root, "__files")
	mustMkdir(tb, mappingsDir)
	mustMkdir(tb, filesDir)
	mustWrite(tb, filepath.Join(filesDir, "payload.json"), `{"ok":true}`)
	mustWrite(tb, filepath.Join(mappingsDir, "perf.yaml"), mappingsYAML(mappings))
	return root
}

func mappingsYAML(count int) string {
	var builder strings.Builder
	builder.WriteString("mappings:\n")
	for i := range count {
		builder.WriteString(mappingYAML(i))
	}
	return builder.String()
}

func mappingYAML(index int) string {
	return fmt.Sprintf("  - id: perf-%03d\n    request:\n      method: GET\n      urlPath: /api/items/%d\n    response:\n      status: 200\n      headers:\n        Content-Type: application/json\n      bodyFileName: payload.json\n", index, index)
}

func runLoad(tb testing.TB, cfg perfConfig, baseURL string) loadSummary {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration)
	defer cancel()
	client := newHTTPClient(cfg)
	defer client.CloseIdleConnections()
	results := make(chan loadResult, cfg.concurrency*2)
	summaryCh := collectResults(results)
	started := time.Now()
	runWorkers(ctx, client, baseURL, cfg, results)
	close(results)
	summary := <-summaryCh
	summary.elapsed = time.Since(started)
	return summary
}

func newHTTPClient(cfg perfConfig) *http.Client {
	transport := &http.Transport{MaxIdleConns: cfg.concurrency * 2, MaxIdleConnsPerHost: cfg.concurrency * 2}
	return &http.Client{Timeout: cfg.requestTimeout, Transport: transport}
}

func collectResults(results <-chan loadResult) <-chan loadSummary {
	summaryCh := make(chan loadSummary, 1)
	go func() {
		summary := newLoadSummary()
		for result := range results {
			summary.observe(result)
		}
		summaryCh <- summary
	}()
	return summaryCh
}

func newLoadSummary() loadSummary {
	return loadSummary{statusCounts: map[int]int64{}, latency: newLatencyHistogram()}
}

func (s *loadSummary) observe(result loadResult) {
	if result.canceled {
		return
	}
	if result.err != nil {
		s.errors++
		return
	}
	s.requests++
	s.statusCounts[result.status]++
	s.latency.observe(result.duration)
	if result.status != http.StatusOK {
		s.nonOK++
	}
}

func runWorkers(ctx context.Context, client *http.Client, baseURL string, cfg perfConfig, results chan<- loadResult) {
	var waitGroup sync.WaitGroup
	for worker := range cfg.concurrency {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			loadWorker(ctx, client, baseURL, cfg, worker, results)
		}()
	}
	waitGroup.Wait()
}

func loadWorker(ctx context.Context, client *http.Client, baseURL string, cfg perfConfig, worker int, results chan<- loadResult) {
	requestIndex := worker
	for ctx.Err() == nil {
		path := fmt.Sprintf("%s/api/items/%d", baseURL, requestIndex%cfg.mappings)
		results <- executeRequest(ctx, client, path)
		requestIndex += cfg.concurrency
	}
}

func executeRequest(ctx context.Context, client *http.Client, url string) loadResult {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return loadResult{err: err}
	}
	started := time.Now()
	response, err := client.Do(request)
	duration := time.Since(started)
	return responseResult(ctx, response, duration, err)
}

func responseResult(ctx context.Context, response *http.Response, duration time.Duration, err error) loadResult {
	if err != nil {
		return loadResult{duration: duration, err: err, canceled: ctx.Err() != nil}
	}
	defer closeBody(response.Body)
	_, _ = io.Copy(io.Discard, response.Body)
	return loadResult{duration: duration, status: response.StatusCode}
}

func runBenchmarkRequest(tb testing.TB, client *http.Client, baseURL string, mappings int, requestIndex int) {
	tb.Helper()
	url := fmt.Sprintf("%s/api/items/%d", baseURL, requestIndex%mappings)
	result := executeRequest(context.Background(), client, url)
	if result.err != nil || result.status != http.StatusOK {
		tb.Fatalf("benchmark request failed: status=%d error=%v", result.status, result.err)
	}
}

func newLatencyHistogram() latencyHistogram {
	return latencyHistogram{buckets: make([]int64, len(latencyBuckets)+1)}
}

func (h *latencyHistogram) observe(duration time.Duration) {
	h.count++
	h.sum += duration
	if h.min == 0 || duration < h.min {
		h.min = duration
	}
	if duration > h.max {
		h.max = duration
	}
	h.buckets[latencyBucketIndex(duration)]++
}

func latencyBucketIndex(duration time.Duration) int {
	for index, bucket := range latencyBuckets {
		if duration <= bucket {
			return index
		}
	}
	return len(latencyBuckets)
}

func (h latencyHistogram) average() time.Duration {
	if h.count == 0 {
		return 0
	}
	return time.Duration(int64(h.sum) / h.count)
}

func (h latencyHistogram) percentile(quantile float64) time.Duration {
	if h.count == 0 {
		return 0
	}
	target := int64(math.Ceil(float64(h.count) * quantile))
	return h.percentileBucket(target)
}

func (h latencyHistogram) percentileBucket(target int64) time.Duration {
	var seen int64
	for index, count := range h.buckets {
		seen += count
		if seen >= target {
			return bucketDuration(index, h.max)
		}
	}
	return h.max
}

func bucketDuration(index int, max time.Duration) time.Duration {
	if index >= len(latencyBuckets) {
		return max
	}
	return latencyBuckets[index]
}

func captureRuntime() runtimeSnapshot {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return runtimeSnapshot{heapAlloc: stats.HeapAlloc, heapInuse: stats.HeapInuse,
		heapObjects: stats.HeapObjects, numGC: stats.NumGC, goroutines: runtime.NumGoroutine()}
}

func logLoadSummary(tb testing.TB, cfg perfConfig, summary loadSummary) {
	tb.Helper()
	tb.Logf("perf smoke: mappings=%d concurrency=%d duration=%s requests=%d rps=%.1f statuses=%s errors=%d non_ok=%d",
		cfg.mappings, cfg.concurrency, summary.elapsed, summary.requests, requestsPerSecond(summary), statusCounts(summary), summary.errors, summary.nonOK)
	tb.Logf("latency: min=%s avg=%s p50=%s p95=%s p99=%s max=%s",
		summary.latency.min, summary.latency.average(), summary.latency.percentile(0.50),
		summary.latency.percentile(0.95), summary.latency.percentile(0.99), summary.latency.max)
}

func logRuntimeSummary(tb testing.TB, before runtimeSnapshot, after runtimeSnapshot) {
	tb.Helper()
	tb.Logf("runtime before: heap_alloc=%s heap_inuse=%s heap_objects=%d goroutines=%d gc=%d",
		bytesText(before.heapAlloc), bytesText(before.heapInuse), before.heapObjects, before.goroutines, before.numGC)
	tb.Logf("runtime after_gc: heap_alloc=%s heap_inuse=%s heap_objects=%d goroutines=%d gc=%d",
		bytesText(after.heapAlloc), bytesText(after.heapInuse), after.heapObjects, after.goroutines, after.numGC)
	tb.Logf("runtime delta: heap_alloc=%s heap_inuse=%s heap_objects=%d goroutines=%d gc=%d",
		bytesDelta(before.heapAlloc, after.heapAlloc), bytesDelta(before.heapInuse, after.heapInuse),
		int64(after.heapObjects)-int64(before.heapObjects), after.goroutines-before.goroutines, after.numGC-before.numGC)
}

func requestsPerSecond(summary loadSummary) float64 {
	if summary.elapsed <= 0 {
		return 0
	}
	return float64(summary.requests) / summary.elapsed.Seconds()
}

func statusCounts(summary loadSummary) string {
	statuses := make([]int, 0, len(summary.statusCounts))
	for status := range summary.statusCounts {
		statuses = append(statuses, status)
	}
	sort.Ints(statuses)
	return joinStatusCounts(statuses, summary.statusCounts)
}

func joinStatusCounts(statuses []int, counts map[int]int64) string {
	parts := make([]string, 0, len(statuses))
	for _, status := range statuses {
		parts = append(parts, fmt.Sprintf("%d=%d", status, counts[status]))
	}
	return strings.Join(parts, ",")
}

func assertLoadSucceeded(tb testing.TB, summary loadSummary) {
	tb.Helper()
	if summary.requests == 0 {
		tb.Fatal("load test did not complete any requests")
	}
	if summary.errors > 0 || summary.nonOK > 0 {
		tb.Fatalf("load test completed with errors=%d non_ok=%d", summary.errors, summary.nonOK)
	}
}

func bytesText(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}
	return fmt.Sprintf("%.2fMiB", float64(value)/(unit*unit))
}

func bytesDelta(before uint64, after uint64) string {
	if after >= before {
		return "+" + bytesText(after-before)
	}
	return "-" + bytesText(before-after)
}
