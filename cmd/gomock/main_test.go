package main

import (
	"context"
	"os"
	"strings"
	"testing"
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
