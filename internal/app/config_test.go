package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidateRejectsNonPositiveVerboseBodyLimit(t *testing.T) {
	config := DefaultConfig()
	config.VerboseBodyLimit = 0

	err := config.Validate()

	if err == nil || !strings.Contains(err.Error(), "verbose-body-limit") {
		fatalUnexpectedValidationError(t, err)
	}
}

func TestConfigValidateRejectsNonPositiveVerbosePreviewLimit(t *testing.T) {
	config := DefaultConfig()
	config.VerbosePreviewLimit = 0

	err := config.Validate()

	if err == nil || !strings.Contains(err.Error(), "verbose-preview-limit") {
		fatalUnexpectedValidationError(t, err)
	}
}

func TestConfigValidateRejectsEnabledTLSWithoutCertFile(t *testing.T) {
	config := DefaultConfig()
	config.TLS.Enabled = true
	config.TLS.KeyFile = "server.key"

	err := config.Validate()

	assertValidationErrorContains(t, err, "tls-cert-file")
}

func TestConfigValidateRejectsEnabledTLSWithoutKeyFile(t *testing.T) {
	config := DefaultConfig()
	config.TLS.Enabled = true
	config.TLS.CertFile = "server.crt"

	err := config.Validate()

	assertValidationErrorContains(t, err, "tls-key-file")
}

func TestConfigValidateRejectsUnreadableTLSFiles(t *testing.T) {
	config := DefaultConfig()
	config.TLS.Enabled = true
	config.TLS.CertFile = filepath.Join(t.TempDir(), "missing.crt")
	config.TLS.KeyFile = writeTLSValidationFile(t)

	err := config.Validate()

	assertValidationErrorContains(t, err, "tls-cert-file", "readable")
}

func TestConfigValidateRejectsInvalidTLSMinVersion(t *testing.T) {
	config := DefaultConfig()
	config.TLS.MinVersion = "1.1"

	err := config.Validate()

	assertValidationErrorContains(t, err, "tls-min-version", "1.2", "1.3")
}

func TestConfigValidateAcceptsEnabledTLSWithReadableFiles(t *testing.T) {
	config := DefaultConfig()
	config.TLS.Enabled = true
	config.TLS.CertFile = writeTLSValidationFile(t)
	config.TLS.KeyFile = writeTLSValidationFile(t)
	config.TLS.MinVersion = "1.3"

	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid TLS config, got %v", err)
	}
}

func writeTLSValidationFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tls-file")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("write TLS validation file: %v", err)
	}
	return path
}

func assertValidationErrorContains(t *testing.T, err error, parts ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("expected %q to contain %q", err.Error(), part)
		}
	}
}

func fatalUnexpectedValidationError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
	t.Fatalf("unexpected validation error: %v", err)
}
