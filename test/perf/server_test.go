package perf

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lHumaNl/gomock/internal/app"
	"github.com/lHumaNl/gomock/internal/configloader"
)

func startGoMock(tb testing.TB, root string) string {
	tb.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	port := freePort(tb)
	application := newApplication(tb, root, port)
	errCh := make(chan error, 1)
	go func() { errCh <- application.Run(ctx) }()
	tb.Cleanup(func() { stopGoMock(tb, cancel, errCh) })
	baseURL := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	waitForReady(tb, baseURL+"/readyz", errCh)
	return baseURL
}

func newApplication(tb testing.TB, root string, port int) *app.Application {
	tb.Helper()
	cfg := app.DefaultConfig()
	cfg.Root = root
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.LogLevel = "error"
	application, err := app.New(cfg, configloader.NewLoader(false), discardLogger())
	if err != nil {
		tb.Fatalf("create application: %v", err)
	}
	return application
}

func stopGoMock(tb testing.TB, cancel context.CancelFunc, errCh <-chan error) {
	tb.Helper()
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			tb.Fatalf("stop gomock: %v", err)
		}
	case <-time.After(startupTimeout):
		tb.Fatalf("stop gomock: %v", context.DeadlineExceeded)
	}
}

func waitForReady(tb testing.TB, url string, errCh <-chan error) {
	tb.Helper()
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if readyURL(url) {
			return
		}
		failIfServerStopped(tb, errCh)
		time.Sleep(25 * time.Millisecond)
	}
	tb.Fatalf("%s did not become ready", url)
}

func readyURL(url string) bool {
	response, err := http.Get(url)
	if err != nil {
		return false
	}
	defer closeBody(response.Body)
	return response.StatusCode == http.StatusOK
}

func failIfServerStopped(tb testing.TB, errCh <-chan error) {
	tb.Helper()
	select {
	case err := <-errCh:
		tb.Fatalf("gomock stopped before readiness: %v", err)
	default:
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func freePort(tb testing.TB) int {
	tb.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen free port: %v", err)
	}
	defer closeListener(listener)
	return listener.Addr().(*net.TCPAddr).Port
}

func mustMkdir(tb testing.TB, path string) {
	tb.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		tb.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(tb testing.TB, path string, content string) {
	tb.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		tb.Fatalf("write %s: %v", path, err)
	}
}

func closeBody(closer io.Closer) {
	_ = closer.Close()
}

func closeListener(listener net.Listener) {
	_ = listener.Close()
}
