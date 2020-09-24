package metrics

import (
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	asrsapi "stash.teslamotors.com/cas/asrs/idl/src"
)

// Counters are exported in case implementation is provided by app to make it possible
// to still count consistently with subset of counters which are common.
type TerminalMetricsHolder struct {
	asrsapi.HelloMetricsHolder
	LoadOperationCounter   *prometheus.CounterVec
	UnloadOperationCounter *prometheus.CounterVec
	AlarmCounter           *prometheus.CounterVec
	// registry, provided by app or default
	registry *prometheus.Registry
	// Are we tracking expensive metrics like summaries and histograms?
	detailed bool
}

// Declare label constants used with metrics
const (
	ResRxFailed        = "rxFailed"
	ResUnmarshalFailed = "unmarshalFailed"
	ResInvalidObject   = "invalidObject"
)

// NewTerminalMetricsHolder allows user (client, client test, server test) to setup common metrics in a way
// which is consistent.
func NewTerminalMetricsHolder(registry *prometheus.Registry, namespace string, detailed bool) *TerminalMetricsHolder {

	if registry == nil {
		var ok bool
		registry, ok = prometheus.DefaultRegisterer.(*prometheus.Registry)
		if !ok {
			return nil
		}
	}

	mh := &TerminalMetricsHolder{
		detailed: detailed,
		registry: registry,
	}

	if detailed {
		// Only works with default registry
		grpc_prometheus.EnableHandlingTimeHistogram()
	}

	mh.HellosCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "terminal",
		Name:      "hellos",
		Help:      "hellos sent and received",
	}, []string{"direction", "result", "remote", "echo"})

	mh.HellosStateChangeCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "terminal",
		Name:      "hellos_statechanges",
		Help:      "state changes triggered by hellos",
	}, []string{"state", "remote"})

	mh.HelloStateGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "terminal",
		Name:      "hellos_state",
		Help:      "state derived from hellos",
	}, []string{"remote"})

	mh.LoadOperationCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "terminal",
		Name:      "load",
		Help:      "load operations sent and received",
	}, []string{"direction", "result", "remote", "ack", "state", "statetype", "status"})

	mh.UnloadOperationCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "terminal",
		Name:      "unload",
		Help:      "unload operations sent and received",
	}, []string{"direction", "result", "remote", "ack", "state", "statetype", "status"})

	mh.AlarmCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "terminal",
		Name:      "alarm",
		Help:      "alarms/acknowledgments sent and received",
	}, []string{"direction", "result", "remote", "status", "level", "location"})

	registry.Register(mh.HellosCounter)
	registry.Register(mh.HellosStateChangeCounter)
	registry.Register(mh.HelloStateGauge)
	registry.Register(mh.LoadOperationCounter)
	registry.Register(mh.UnloadOperationCounter)
	registry.Register(mh.AlarmCounter)

	return mh
}

func (mh *TerminalMetricsHolder) GetHelloMetricsHolder() *asrsapi.HelloMetricsHolder {
	if mh == nil {
		return nil
	}
	return &mh.HelloMetricsHolder
}
