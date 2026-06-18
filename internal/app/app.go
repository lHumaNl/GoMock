package app

import (
	"context"
	"log/slog"
	"net"
	"strconv"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/domain/stub"
	"github.com/lHumaNl/gomock/internal/observability"
	"github.com/lHumaNl/gomock/internal/server"
)

type MappingLoader interface {
	LoadRoot(root string) ([]mapping.Mapping, error)
}

type Application struct {
	config Config
	loader MappingLoader
	logger *slog.Logger
}

func New(config Config, loader MappingLoader, logger *slog.Logger) (*Application, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Application{config: config, loader: loader, logger: logger}, nil
}

func (a *Application) Run(ctx context.Context) error {
	mappings, err := a.loader.LoadRoot(a.config.Root)
	if err != nil {
		return err
	}
	metrics, err := a.newMetrics(len(mappings))
	if err != nil {
		return err
	}

	a.logger.InfoContext(ctx, "mappings loaded",
		"root", a.config.Root,
		"count", len(mappings),
		"host", a.config.Host,
		"port", a.config.Port,
		"metrics_port", a.config.MetricsPort,
	)
	service := stub.NewService(mappings)
	handler := a.newMainHandler(service, metrics)
	servers := []*server.HTTPServer{server.NewHTTPServer(a.address(), handler, a.logger)}
	if a.separateMetricsServer() {
		metricsHandler := server.NewMetricsHandler(metrics.Handler())
		servers = append(servers, server.NewHTTPServer(a.metricsAddress(), metricsHandler, a.logger))
	}
	return runServers(ctx, servers)
}

func (a *Application) address() string {
	return a.addressForPort(a.config.Port)
}

func (a *Application) metricsAddress() string {
	return a.addressForPort(a.config.MetricsPort)
}

func (a *Application) addressForPort(port int) string {
	return net.JoinHostPort(a.config.Host, strconv.Itoa(port))
}

func (a *Application) newMetrics(mappingCount int) (*observability.Metrics, error) {
	build := observability.BuildInfo{Version: a.config.Version, Commit: a.config.Commit}
	metrics, err := observability.NewMetrics(nil, build)
	if err != nil {
		return nil, err
	}
	metrics.SetMappingsLoaded(mappingCount)
	return metrics, nil
}

func (a *Application) newMainHandler(service *stub.Service, metrics *observability.Metrics) *server.Handler {
	if a.separateMetricsServer() {
		return server.NewHandlerWithOptions(service, alwaysReady, a.logger, metrics, nil, a.verboseConfig())
	}
	return server.NewHandlerWithOptions(service, alwaysReady, a.logger, metrics, metrics.Handler(), a.verboseConfig())
}

func (a *Application) verboseConfig() server.VerboseConfig {
	return server.VerboseConfig{
		Mode:         a.config.Verbose,
		BodyLimit:    a.config.VerboseBodyLimit,
		PreviewLimit: a.config.VerbosePreviewLimit,
		Redact:       a.config.VerboseRedact,
	}
}

func (a *Application) separateMetricsServer() bool {
	return a.config.MetricsPort != 0 && a.config.MetricsPort != a.config.Port
}

func runServers(ctx context.Context, servers []*server.HTTPServer) error {
	groupCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errorsChan := make(chan error, len(servers))
	for _, item := range servers {
		go func() { errorsChan <- item.Run(groupCtx) }()
	}
	return waitForServers(errorsChan, cancel, len(servers))
}

func waitForServers(errorsChan <-chan error, cancel context.CancelFunc, count int) error {
	var firstErr error
	for range count {
		err := <-errorsChan
		if err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
	}
	return firstErr
}

func alwaysReady() bool {
	return true
}
