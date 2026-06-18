package perf

import (
	"os"
	"runtime/pprof"
	"testing"
)

func startCPUProfile(tb testing.TB, path string) func() {
	tb.Helper()
	if path == "" {
		return func() {}
	}
	file := createProfileFile(tb, path)
	if err := pprof.StartCPUProfile(file); err != nil {
		closeBody(file)
		tb.Fatalf("start CPU profile: %v", err)
	}
	return func() { pprof.StopCPUProfile(); closeBody(file) }
}

func writeMemoryProfile(tb testing.TB, path string) {
	tb.Helper()
	if path == "" {
		return
	}
	file := createProfileFile(tb, path)
	defer closeBody(file)
	if err := pprof.WriteHeapProfile(file); err != nil {
		tb.Fatalf("write heap profile: %v", err)
	}
}

func createProfileFile(tb testing.TB, path string) *os.File {
	tb.Helper()
	file, err := os.Create(path)
	if err != nil {
		tb.Fatalf("create profile %s: %v", path, err)
	}
	return file
}
