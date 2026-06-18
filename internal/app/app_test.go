package app

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
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

func TestApplicationRunStartsHTTPSServerWithSelfSignedCert(t *testing.T) {
	root := newAppTestRoot(t)
	writeAppFile(t, root, "mappings/secure.yaml", "request:\n  method: GET\n  urlPath: /secure\nresponse:\n  status: 200\n  body: secure\n")
	cert := writeSelfSignedCert(t)
	port := freePort(t)
	config := Config{Root: root, Host: "127.0.0.1", Port: port, LogLevel: DefaultLogLevel,
		TLS: TLSConfig{Enabled: true, CertFile: cert.certFile, KeyFile: cert.keyFile, MinVersion: DefaultTLSMinVersion}}
	application := newTestApplication(t, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errorsChan := runApplication(ctx, application)
	client := newTLSClient(cert.pool)
	baseURL := "https://127.0.0.1:" + strconv.Itoa(port)
	waitForOKWithClient(t, client, baseURL+"/readyz")
	waitForOKWithClient(t, client, baseURL+"/metrics")

	response, err := client.Get(baseURL + "/secure")
	if err != nil {
		t.Fatalf("get HTTPS stub response: %v", err)
	}
	defer closeBody(response.Body)
	assertAppResponse(t, response, http.StatusOK, "secure")
	cancel()
	assertApplicationStopped(t, errorsChan)
}

type testCertificate struct {
	certFile string
	keyFile  string
	pool     *x509.CertPool
}

func newTestApplication(t *testing.T, config Config) *Application {
	t.Helper()
	if config.VerboseBodyLimit == 0 {
		config.VerboseBodyLimit = DefaultVerboseBodyLimit
	}
	if config.VerbosePreviewLimit == 0 {
		config.VerbosePreviewLimit = DefaultVerbosePreviewLimit
	}
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
	waitForOKWithClient(t, http.DefaultClient, url)
}

func waitForOKWithClient(t *testing.T, client *http.Client, url string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ready(client, url) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not become ready at %s", url)
}

func ready(client *http.Client, url string) bool {
	response, err := client.Get(url)
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

func writeSelfSignedCert(t *testing.T) testCertificate {
	t.Helper()
	certPEM, keyPEM := generateSelfSignedCert(t)
	root := t.TempDir()
	certFile := filepath.Join(root, "server.crt")
	keyFile := filepath.Join(root, "server.key")
	writeAppFile(t, root, "server.crt", string(certPEM))
	writeAppFile(t, root, "server.key", string(keyPEM))
	return testCertificate{certFile: certFile, keyFile: keyFile, pool: certPool(t, certPEM)}
}

func generateSelfSignedCert(t *testing.T) ([]byte, []byte) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	certDER := createCertificate(t, privateKey)
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return pemBlock("CERTIFICATE", certDER), pemBlock("EC PRIVATE KEY", keyDER)
}

func createCertificate(t *testing.T, privateKey *ecdsa.PrivateKey) []byte {
	t.Helper()
	template := certificateTemplate()
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return certDER
}

func certificateTemplate() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
}

func pemBlock(blockType string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
}

func certPool(t *testing.T, certPEM []byte) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certPEM) {
		t.Fatal("append cert to pool")
	}
	return pool
}

func newTLSClient(pool *x509.CertPool) *http.Client {
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
}
