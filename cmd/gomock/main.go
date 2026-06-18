package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lHumaNl/gomock/internal/app"
	"github.com/lHumaNl/gomock/internal/configloader"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout *os.File, stderr *os.File) int {
	config, showVersion, err := parseFlags(args, stdout, stderr)
	if err != nil {
		return 2
	}
	if showVersion {
		_, _ = fmt.Fprintln(stdout, version)
		return 0
	}

	logger := newLogger(stderr, config.LogLevel)
	application, err := app.New(config, configloader.NewLoader(config.Strict), logger)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	if err := application.Run(ctx); err != nil {
		logger.Error("startup failed", "error", err)
		return 1
	}
	return 0
}

func parseFlags(args []string, stdout *os.File, stderr *os.File) (app.Config, bool, error) {
	config := app.DefaultConfig()
	config.Version = version
	config.Commit = commit
	flags := flag.NewFlagSet("gomock", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print version and exit")
	flags.StringVar(&config.Root, "root", config.Root, "mock root directory")
	flags.StringVar(&config.Host, "host", config.Host, "HTTP bind host")
	flags.IntVar(&config.Port, "port", config.Port, "HTTP bind port")
	flags.IntVar(&config.MetricsPort, "metrics-port", config.MetricsPort, "optional metrics port")
	flags.StringVar(&config.LogLevel, "log-level", config.LogLevel, "debug, info, warn, or error")
	flags.BoolVar(&config.Strict, "strict", config.Strict, "reject unknown mapping fields")
	flags.StringVar(&config.Verbose, "verbose", config.Verbose, "traffic logs: off, summary, or full")
	flags.IntVar(&config.VerboseBodyLimit, "verbose-body-limit", config.VerboseBodyLimit, "max request/response body bytes in full traffic logs")
	flags.IntVar(&config.VerbosePreviewLimit, "verbose-preview-limit", config.VerbosePreviewLimit, "max request URI characters in summary traffic logs")
	flags.BoolVar(&config.VerboseRedact, "verbose-redact", config.VerboseRedact, "redact sensitive headers, query parameters, and body fields in traffic logs")
	err := flags.Parse(args)
	return config, *showVersion, err
}

func newLogger(output *os.File, level string) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: slogLevel}))
}
