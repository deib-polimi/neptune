package monitoring

import (
	"sync"

	"github.com/asecurityteam/rolling"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/backend"
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

// FunctionMetrics is the wrapper for the metrics related to a specific function
type FunctionMetrics struct {
	function string
	metrics  map[MetricType]*rolling.TimePolicy
}

// DataStore is the main data structure holding all the metrics
type DataStore struct {
	metricChan <-chan Metric
	cacheLock  sync.RWMutex
	cache      map[backend.Backend]float64
	metrics    map[string]FunctionMetrics
}

// NewDataStore returns a new DataStore
func NewDataStore(metricChan <-chan Metric) *DataStore {
	return &DataStore{
		metricChan: metricChan,
	}
}
