package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lHumaNl/gomock/internal/app"
)

func TestParseFlagsHelpOmitsVerboseIncludeProbes(t *testing.T) {
	stdout := tempFile(t)
	stderr := tempFile(t)
	defer closeFile(stdout)
	defer closeFile(stderr)

	_, _, err := parseFlags([]string{"-h"}, stdout, stderr)

	if err == nil {
		t.Fatal("expected help error")
	}
	help := readFile(t, stderr)
	if strings.Contains(help, "verbose-include-probes") {
		t.Fatalf("help should not include removed flag: %s", help)
	}
}

func TestRunRejectsVerboseIncludeProbesFlag(t *testing.T) {
	stdout := tempFile(t)
	stderr := tempFile(t)
	defer closeFile(stdout)
	defer closeFile(stderr)

	code := run(context.Background(), []string{"--verbose-include-probes"}, stdout, stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	content := readFile(t, stderr)
	if !strings.Contains(content, "flag provided but not defined") {
		t.Fatalf("expected unknown flag error, got %s", content)
	}
}

func TestParseFlagsAppliesPartialConfigDefaults(t *testing.T) {
	path := writeMainTestFile(t, "gomock.yaml", "host: 127.0.0.1\n")

	config := parseConfig(t, "--config", path)

	if config.Host != "127.0.0.1" || config.Port != app.DefaultPort {
		t.Fatalf("unexpected partial config: %#v", config)
	}
	if config.Root != app.DefaultRoot || config.TLS.MinVersion != app.DefaultTLSMinVersion {
		t.Fatalf("defaults were not preserved: %#v", config)
	}
}

func TestParseFlagsLetsCLIOverrideConfigFile(t *testing.T) {
	path := writeMainTestFile(t, "gomock.yaml", cliOverrideConfig())

	config := parseConfig(t, "--config", path, "--port", "9090", "--tls=false")

	if config.Port != 9090 || config.TLS.Enabled {
		t.Fatalf("CLI override was not applied: %#v", config)
	}
	if config.Root != "/from-config" || config.TLS.MinVersion != "1.3" {
		t.Fatalf("config values were not preserved: %#v", config)
	}
}

func TestParseFlagsLoadsYAMLConfigFile(t *testing.T) {
	path := writeMainTestFile(t, "gomock.yml", yamlStartupConfig())

	config := parseConfig(t, "--config="+path)

	assertStartupConfig(t, config, "/yaml", 8081, "debug")
}

func TestParseFlagsLoadsJSON5ConfigFile(t *testing.T) {
	path := writeMainTestFile(t, "gomock.json5", json5StartupConfig())

	config := parseConfig(t, "--config", path)

	assertStartupConfig(t, config, "/json5", 8082, "warn")
}

func parseConfig(t *testing.T, args ...string) app.Config {
	t.Helper()
	stdout := tempFile(t)
	stderr := tempFile(t)
	defer closeFile(stdout)
	defer closeFile(stderr)
	config, _, err := parseFlags(args, stdout, stderr)
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return config
}

func assertStartupConfig(t *testing.T, config app.Config, root string, port int, level string) {
	t.Helper()
	if config.Root != root || config.Port != port || config.LogLevel != level {
		t.Fatalf("unexpected startup config: %#v", config)
	}
	if !config.TLS.Enabled || config.TLS.CertFile == "" || config.TLS.KeyFile == "" {
		t.Fatalf("expected TLS values from config: %#v", config.TLS)
	}
}

func cliOverrideConfig() string {
	return "root: /from-config\nport: 8088\ntls:\n  enabled: true\n  minVersion: '1.3'\n"
}

func yamlStartupConfig() string {
	return "root: /yaml\nport: 8081\nlogLevel: debug\ntls:\n  enabled: true\n  certFile: cert.pem\n  keyFile: key.pem\n"
}

func json5StartupConfig() string {
	return `{
		// JSON5 features are allowed.
		root: '/json5',
		port: 8082,
		logLevel: 'warn',
		tls: {enabled: true, certFile: 'cert.pem', keyFile: 'key.pem'},
	}`
}

func writeMainTestFile(t *testing.T, name string, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write main test file: %v", err)
	}
	return path
}

func tempFile(t *testing.T) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "gomock-flags-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	return file
}

func readFile(t *testing.T, file *os.File) string {
	t.Helper()
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("seek temp file: %v", err)
	}
	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	return string(content)
}

func closeFile(file *os.File) {
	_ = file.Close()
}
