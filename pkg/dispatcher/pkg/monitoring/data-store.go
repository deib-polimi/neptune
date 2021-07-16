package monitoring

import (
	"sync"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
)

// MetricType are the defined metrics offered by the monitoring component
type MetricType string

const (
	// ResponseTime is the response time of the function
	ResponseTime MetricType = "response-time"
	// RequestCount is the number of requests forwarded to the function
	RequestCount MetricType = "request-count"
	// Throughput is the throughput of the function
	Throughput MetricType = "throughput"
)

// BackendList is used to update the backends associated to a function
type BackendList struct {
	// FunctionURL is the URL of the function invo
	FunctionURL string
	// Backends are the backends that serves a function
	Backends []string
}

// DataStore is the main data structure holding all the metrics
type DataStore struct {
	backendChan      <-chan BackendList
	metricChan       <-chan metrics.RawMetricData
	metricsLock      sync.RWMutex
	metrics          map[string]*metrics.FunctionMetrics
	windowParameters metrics.WindowParameters
}

// NewDataStore returns a new DataStore
func NewDataStore(backendChan <-chan BackendList, metricChan <-chan metrics.RawMetricData) *DataStore {
	return &DataStore{
		backendChan: backendChan,
		metricChan:  metricChan,
		metricsLock: sync.RWMutex{},
		metrics:     make(map[string]*metrics.FunctionMetrics),
		// TODO: customize with env var?
		windowParameters: metrics.WindowParameters{
			WindowSize:        1 * time.Minute,
			WindowGranularity: 30 * time.Second,
		},
	}
}

func (ds *DataStore) Poll() {
	go func() {
		for {
			select {
			case raw := <-ds.metricChan:
				ds.handleRawData(raw)
			case backends := <-ds.backendChan:
				ds.updateBackends(backends.FunctionURL, backends.Backends)
			default:
			}
		}
	}()
}

func (ds *DataStore) handleRawData(metric metrics.RawMetricData) {
	ds.metricsLock.Lock()
	defer ds.metricsLock.Unlock()

	var ms *metrics.FunctionMetrics
	var exists bool

	if ms, exists = ds.metrics[metric.FunctionURL]; !exists {
		ms = metrics.NewFunctionMetrics(ds.windowParameters)
		ds.metrics[metric.FunctionURL] = ms
	}

	ms.SetBackend(metric.Backend)

	backend, _ := ms.GetBackend(metric.Backend)
	backend.AddValue(metric.Value)
}

func (ds *DataStore) updateBackends(function string, backends []string) {
	ds.metricsLock.Lock()
	defer ds.metricsLock.Unlock()

	functionBackends := ds.metrics[function]

	// add new backends
	for _, b := range backends {
		if _, exists := functionBackends.GetBackend(b); !exists {
			functionBackends.SetBackend(b)
		}
	}

	// delete old backends

}
