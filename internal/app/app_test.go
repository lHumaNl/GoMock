package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lHumaNl/gomock/internal/configloader"
)

func TestApplicationRunStartsHTTPServerWithLoadedMappings(t *testing.T) {
	root := newAppTestRoot(t)
	writeAppFile(t, root, "mappings/ping.yaml", "request:\n  method: GET\n  urlPath: /ping\nresponse:\n  status: 200\n  body: pong\n")
	port := freePort(t)
	config := Config{Root: root, Host: "127.0.0.1", Port: port, LogLevel: DefaultLogLevel}
	application := newTestApplication(t, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errorsChan := runApplication(ctx, application)
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	waitForOK(t, baseURL+"/readyz")

	response, err := http.Get(baseURL + "/ping")
	if err != nil {
		t.Fatalf("get stub response: %v", err)
	}
	defer closeBody(response.Body)
	assertAppResponse(t, response, http.StatusOK, "pong")
	cancel()
	assertApplicationStopped(t, errorsChan)
}

func TestApplicationExposesMetricsOnMainPortByDefault(t *testing.T) {
	root := newAppTestRoot(t)
	writeAppFile(t, root, "mappings/ping.yaml", "request:\n  method: GET\n  urlPath: /ping\nresponse:\n  status: 200\n  body: pong\n")
	port := freePort(t)
	application := newTestApplication(t, Config{Root: root, Host: "127.0.0.1", Port: port, LogLevel: DefaultLogLevel})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errorsChan := runApplication(ctx, application)
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	waitForOK(t, baseURL+"/readyz")
	waitForOK(t, baseURL+"/metrics")

	metrics := getContent(t, baseURL+"/metrics")
	assertContains(t, metrics, "gomock_mappings_loaded 1")
	cancel()
	assertApplicationStopped(t, errorsChan)
}

func TestApplicationRunsSeparateMetricsServerWhenConfigured(t *testing.T) {
	root := newAppTestRoot(t)
	port := freePort(t)
	metricsPort := freePort(t)
	config := Config{Root: root, Host: "127.0.0.1", Port: port,
		MetricsPort: metricsPort, LogLevel: DefaultLogLevel}
	application := newTestApplication(t, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errorsChan := runApplication(ctx, application)
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	metricsURL := "http://127.0.0.1:" + strconv.Itoa(metricsPort) + "/metrics"
	waitForOK(t, baseURL+"/readyz")
	waitForOK(t, metricsURL)

	response := getResponse(t, baseURL+"/metrics")
	defer closeBody(response.Body)
	assertAppResponse(t, response, http.StatusNotFound, "{\"error\":\"No matching stub found\",\"method\":\"GET\",\"path\":\"/metrics\"}\n")
	assertContains(t, getContent(t, metricsURL), "gomock_mappings_loaded 0")
	cancel()
	assertApplicationStopped(t, errorsChan)
}

func newTestApplication(t *testing.T, config Config) *Application {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := New(config, configloader.NewLoader(false), logger)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	return application
}

func runApplication(ctx context.Context, application *Application) <-chan error {
	errorsChan := make(chan error, 1)
	go func() { errorsChan <- application.Run(ctx) }()
	return errorsChan
}

func waitForOK(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ready(url) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not become ready at %s", url)
}

func ready(url string) bool {
	response, err := http.Get(url)
	if err != nil {
		return false
	}
	defer closeBody(response.Body)
	return response.StatusCode == http.StatusOK
}

func getContent(t *testing.T, url string) string {
	t.Helper()
	response := getResponse(t, url)
	defer closeBody(response.Body)
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return string(content)
}

func getResponse(t *testing.T, url string) *http.Response {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	return response
}

func assertContains(t *testing.T, content string, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("expected %q to contain %q", content, want)
	}
}

func assertApplicationStopped(t *testing.T, errorsChan <-chan error) {
	t.Helper()
	select {
	case err := <-errorsChan:
		if err != nil {
			t.Fatalf("application stopped with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("application did not stop")
	}
}

func assertAppResponse(t *testing.T, response *http.Response, status int, body string) {
	t.Helper()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if response.StatusCode != status || string(content) != body {
		t.Fatalf("expected %d %q, got %d %q", status, body, response.StatusCode, content)
	}
}

func newAppTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mappings"), 0o755); err != nil {
		t.Fatalf("mkdir mappings: %v", err)
	}
	return root
}

func writeAppFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write app file: %v", err)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer closeListener(listener)
	return listener.Addr().(*net.TCPAddr).Port
}

func closeBody(body io.Closer) {
	_ = body.Close()
}

func closeListener(listener net.Listener) {
	_ = listener.Close()
}
