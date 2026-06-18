package observability

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultVersion = "dev"
	defaultCommit  = "unknown"
)

var requestMetricLabels = []string{"stub", "variant", "method", "status", "matched"}

type BuildInfo struct {
	Version string
	Commit  string
}

type Metrics struct {
	requestsTotal  *prometheus.CounterVec
	requestLatency *prometheus.HistogramVec
	inFlight       prometheus.Gauge
	mappingsLoaded prometheus.Gauge
	buildInfo      *prometheus.GaugeVec
	handler        http.Handler
}

func NewMetrics(registry *prometheus.Registry, build BuildInfo) (*Metrics, error) {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	metrics := newMetrics(registry)
	if err := metrics.register(registry); err != nil {
		return nil, err
	}
	metrics.setBuildInfo(build)
	return metrics, nil
}

func (m *Metrics) Handler() http.Handler {
	return m.handler
}

func (m *Metrics) SetMappingsLoaded(count int) {
	m.mappingsLoaded.Set(float64(count))
}

func (m *Metrics) RequestStarted() {
	m.inFlight.Inc()
}

func (m *Metrics) RequestFinished(
	stub string,
	variant string,
	method string,
	status int,
	matched bool,
	duration time.Duration,
) {
	labels := metricLabelValues(stub, variant, method, status, matched)
	m.requestsTotal.WithLabelValues(labels...).Inc()
	m.requestLatency.WithLabelValues(labels...).Observe(duration.Seconds())
	m.inFlight.Dec()
}

func newMetrics(registry *prometheus.Registry) *Metrics {
	return &Metrics{
		requestsTotal:  newRequestsTotal(),
		requestLatency: newRequestLatency(),
		inFlight:       newInFlightGauge(),
		mappingsLoaded: newMappingsLoadedGauge(),
		buildInfo:      newBuildInfoGauge(),
		handler:        promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	}
}

func (m *Metrics) register(registry *prometheus.Registry) error {
	collectors := []prometheus.Collector{
		m.requestsTotal,
		m.requestLatency,
		m.inFlight,
		m.mappingsLoaded,
		m.buildInfo,
	}
	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

func newRequestsTotal() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "gomock_requests_total",
		Help: "Total number of handled stub requests.",
	}, requestMetricLabels)
}

func newRequestLatency() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gomock_request_duration_seconds",
		Help:    "Stub request handling duration including configured delay.",
		Buckets: prometheus.DefBuckets,
	}, requestMetricLabels)
}

func newInFlightGauge() prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gomock_inflight_requests",
		Help: "Number of stub requests currently being handled.",
	})
}

func newMappingsLoadedGauge() prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gomock_mappings_loaded",
		Help: "Number of mappings loaded at startup.",
	})
}

func newBuildInfoGauge() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gomock_build_info",
		Help: "Build information for the GoMock binary.",
	}, []string{"version", "commit", "go_version"})
}

func (m *Metrics) setBuildInfo(build BuildInfo) {
	build = normalizeBuildInfo(build)
	m.buildInfo.WithLabelValues(build.Version, build.Commit, runtime.Version()).Set(1)
}

func normalizeBuildInfo(build BuildInfo) BuildInfo {
	if build.Version == "" {
		build.Version = defaultVersion
	}
	if build.Commit == "" {
		build.Commit = defaultCommit
	}
	return build
}

func metricLabelValues(stub string, variant string, method string, status int, matched bool) []string {
	return []string{stub, variant, method, strconv.Itoa(status), strconv.FormatBool(matched)}
}
