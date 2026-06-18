package e2e

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const processTimeout = 5 * time.Second

func TestGoMockBinaryServesMappingsMetricsAndShutsDown(t *testing.T) {
	binary := buildBinary(t)
	root := newMockRoot(t)
	mainPort := freePort(t)
	metricsPort := freePort(t)
	process := startGoMock(t, binary, root, mainPort, metricsPort)
	defer process.stop(t)

	baseURL := "http://127.0.0.1:" + strconv.Itoa(mainPort)
	metricsURL := "http://127.0.0.1:" + strconv.Itoa(metricsPort) + "/metrics"
	waitForStatus(t, baseURL+"/readyz", http.StatusOK)

	assertUsersResponse(t, baseURL)
	assertSequentialResponses(t, baseURL)
	assertUnmatchedResponse(t, baseURL)
	assertMetrics(t, metricsURL)
	process.stop(t)
}

type runningProcess struct {
	command  *exec.Cmd
	stdout   *bytes.Buffer
	stderr   *bytes.Buffer
	done     chan error
	stopErr  error
	stopOnce sync.Once
}

func buildBinary(t *testing.T) string {
	t.Helper()
	root := repositoryRoot(t)
	binary := filepath.Join(t.TempDir(), binaryName())
	command := exec.Command("go", "build", "-o", binary, "./cmd/gomock")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build gomock binary: %v\n%s", err, output)
	}
	return binary
}

func startGoMock(t *testing.T, binary string, root string, port int, metricsPort int) *runningProcess {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := exec.CommandContext(ctx, binary, "--root", root, "--host", "127.0.0.1",
		"--port", strconv.Itoa(port), "--metrics-port", strconv.Itoa(metricsPort), "--log-level", "error")
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start gomock: %v", err)
	}
	process := &runningProcess{command: command, stdout: stdout, stderr: stderr, done: make(chan error, 1)}
	go func() { process.done <- command.Wait() }()
	return process
}

func (p *runningProcess) stop(t *testing.T) {
	t.Helper()
	p.stopOnce.Do(func() { p.stopErr = p.signalAndWait() })
	if p.stopErr != nil {
		t.Fatalf("gomock shutdown failed: %v\nstdout:\n%s\nstderr:\n%s", p.stopErr, p.stdout.String(), p.stderr.String())
	}
}

func (p *runningProcess) signalAndWait() error {
	select {
	case err := <-p.done:
		return err
	default:
	}
	if err := p.command.Process.Signal(os.Interrupt); err != nil && !strings.Contains(err.Error(), "process already finished") {
		return err
	}
	select {
	case err := <-p.done:
		return err
	case <-time.After(processTimeout):
		_ = p.command.Process.Kill()
		return context.DeadlineExceeded
	}
}

func newMockRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "mappings"))
	mustMkdir(t, filepath.Join(root, "__files"))
	writeFile(t, root, "__files/users.json", `{"users":[{"id":1,"name":"Ada"}]}`)
	writeFile(t, root, "mappings/users.yaml", usersMapping())
	writeFile(t, root, "mappings/pages.yaml", pagesMapping())
	return root
}

func usersMapping() string {
	return "id: get-users\nrequest:\n  method: GET\n  urlPath: /api/users\n  queryParameters:\n    active:\n      equalTo: \"true\"\n  headers:\n    X-Client:\n      contains: web\nresponse:\n  status: 200\n  headers:\n    Content-Type: application/json\n  bodyFileName: users.json\n"
}

func pagesMapping() string {
	return "id: pages\nrequest:\n  method: GET\n  urlPath: /api/pages\nresponses:\n  mode: sequential\n  variants:\n    - name: first\n      status: 200\n      body: '{\"page\":1}'\n    - name: second\n      status: 202\n      body: '{\"page\":2}'\n"
}

func assertUsersResponse(t *testing.T, baseURL string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, baseURL+"/api/users?active=true", nil)
	if err != nil {
		t.Fatalf("new users request: %v", err)
	}
	request.Header.Set("X-Client", "web-app")
	response := doRequest(t, request)
	assertHTTPResponse(t, response, http.StatusOK, `{"users":[{"id":1,"name":"Ada"}]}`)
	if response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected JSON content type, got %q", response.Header.Get("Content-Type"))
	}
}

func assertSequentialResponses(t *testing.T, baseURL string) {
	t.Helper()
	first := getResponse(t, baseURL+"/api/pages")
	assertHTTPResponse(t, first, http.StatusOK, `{"page":1}`)
	second := getResponse(t, baseURL+"/api/pages")
	assertHTTPResponse(t, second, http.StatusAccepted, `{"page":2}`)
}

func assertUnmatchedResponse(t *testing.T, baseURL string) {
	t.Helper()
	response := getResponse(t, baseURL+"/missing")
	assertHTTPResponseContains(t, response, http.StatusNotFound, "No matching stub found")
}

func assertMetrics(t *testing.T, metricsURL string) {
	t.Helper()
	metrics := responseBody(t, getResponse(t, metricsURL))
	assertContains(t, metrics, "gomock_mappings_loaded 2")
	assertMetricLabels(t, metrics, []string{`stub="get-users"`, `variant="default"`, `status="200"`, `matched="true"`})
	assertMetricLabels(t, metrics, []string{`stub="pages"`, `variant="first"`, `status="200"`, `matched="true"`})
	assertMetricLabels(t, metrics, []string{`stub="unmatched"`, `variant="default"`, `status="404"`, `matched="false"`})
}

func waitForStatus(t *testing.T, url string, status int) {
	t.Helper()
	deadline := time.Now().Add(processTimeout)
	for time.Now().Before(deadline) {
		response, err := http.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == status {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s did not return %d", url, status)
}

func getResponse(t *testing.T, url string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new GET request: %v", err)
	}
	return doRequest(t, request)
}

func doRequest(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	client := http.Client{Timeout: processTimeout}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("do %s %s: %v", request.Method, request.URL, err)
	}
	return response
}

func assertHTTPResponse(t *testing.T, response *http.Response, status int, body string) {
	t.Helper()
	content := responseBody(t, response)
	if response.StatusCode != status || content != body {
		t.Fatalf("expected %d %q, got %d %q", status, body, response.StatusCode, content)
	}
}

func assertHTTPResponseContains(t *testing.T, response *http.Response, status int, want string) {
	t.Helper()
	content := responseBody(t, response)
	if response.StatusCode != status || !strings.Contains(content, want) {
		t.Fatalf("expected %d containing %q, got %d %q", status, want, response.StatusCode, content)
	}
}

func responseBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer func() { _ = response.Body.Close() }()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(content)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}
	return findGoModRoot(t, filepath.Dir(file))
}

func findGoModRoot(t *testing.T, start string) string {
	t.Helper()
	for dir := start; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
	}
	t.Fatalf("go.mod not found from %s", start)
	return ""
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "gomock.exe"
	}
	return "gomock"
}

func assertContains(t *testing.T, content string, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("expected content to contain %q\ncontent:\n%s", want, content)
	}
}

func assertMetricLabels(t *testing.T, metrics string, labels []string) {
	t.Helper()
	for _, line := range strings.Split(metrics, "\n") {
		if hasMetricLabels(line, labels) {
			return
		}
	}
	t.Fatalf("expected metric labels %v\nmetrics:\n%s", labels, metrics)
}

func hasMetricLabels(line string, labels []string) bool {
	if !strings.HasPrefix(line, "gomock_requests_total") {
		return false
	}
	for _, label := range labels {
		if !strings.Contains(line, label) {
			return false
		}
	}
	return true
}
