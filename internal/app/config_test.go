package app

import (
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

func fatalUnexpectedValidationError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
	t.Fatalf("unexpected validation error: %v", err)
}
