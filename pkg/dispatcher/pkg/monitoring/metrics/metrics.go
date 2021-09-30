package metrics

import (
	"math"
	"net/url"
	"sync"
	"time"

	"github.com/asecurityteam/rolling"
)

// RawResponseTime is the response time for a single http request
type RawResponseTime struct {
	Timestamp   time.Time
	Source      string
	Destination string
	Function    string
	Namespace   string
	Community   string
	Gpu         bool
	Latency     int
	StatusCode  int
	Description string
}

func (r RawResponseTime) AsCopy() []interface{} {
	return []interface{}{
		r.Timestamp,
		r.Source,
		r.Destination,
		r.Function,
		r.Namespace,
		r.Community,
		r.Gpu,
		r.Latency,
		r.StatusCode,
		r.Description,
	}
}

type RawResourceData struct {
	Timestamp time.Time
	Node      string
	Function  string
	Namespace string
	Community string
	Cores     int
}

func (r RawResourceData) AsCopy() []interface{} {
	return []interface{}{
		r.Timestamp,
		r.Node,
		r.Function,
		r.Namespace,
		r.Community,
		r.Cores,
	}
}

// ExposedMetrics is a struct that wraps the exposed metrics
type ExposedMetrics struct {
	ResponseTime float64 `json:"response-time"`
	RequestCount float64 `json:"request-count"`
	Throughput   float64 `json:"throughput"`
}

// RawMetricData represents the single data point collected by each load balancer
type RawMetricData struct {
	// Backend is the pod which has processed the request
	Backend *url.URL
	// Value is the response time of the request
	Value float64
	// FunctionURL is the URL of the function invo
	FunctionURL string
}

// WindowParameters wraps the parameters used to define the time window on which
// metrics are calculated.
type WindowParameters struct {
	WindowSize        time.Duration
	WindowGranularity time.Duration
}

// BackendMetrics is the wrapper for the metrics related to a specific backend
type BackendMetrics struct {
	window *rolling.TimePolicy
	WindowParameters
}

// NewBackendMetrics returns a new backend metrics
func NewBackendMetrics(params WindowParameters) *BackendMetrics {
	bm := &BackendMetrics{
		WindowParameters: params,
	}
	bm.window = bm.newMetricWindow()
	return bm
}

func (b *BackendMetrics) newMetricWindow() *rolling.TimePolicy {
	return rolling.NewTimePolicy(rolling.NewWindow(int(b.WindowSize.Nanoseconds()/b.WindowGranularity.Nanoseconds())), time.Millisecond)
}

// AddValue adds a new value to the backend metrics
func (b *BackendMetrics) AddValue(v float64) {
	b.window.Append(v)
}

// ResponseTime return the pod average response time
func (b BackendMetrics) ResponseTime() (responseTime float64) {
	responseTime = b.window.Reduce(rolling.Avg)
	// TODO: should we keep 0 or NaN?
	if math.IsNaN(responseTime) {
		responseTime = 0
	}
	return
}

// RequestCount return the current number of request sent to the pod
func (b BackendMetrics) RequestCount() (requestCount float64) {
	requestCount = b.window.Reduce(rolling.Count)
	if math.IsNaN(requestCount) {
		requestCount = 0
	}
	return
}

// Throughput returns the pod throughput in request per second
func (b BackendMetrics) Throughput() (throughput float64) {
	throughput = b.window.Reduce(rolling.Count) / b.WindowSize.Seconds()
	if math.IsNaN(throughput) {
		throughput = 0
	}
	return
}

// metrics returns the exposed metrics
func (b BackendMetrics) metrics() *ExposedMetrics {
	return &ExposedMetrics{
		ResponseTime: b.ResponseTime(),
		RequestCount: b.RequestCount(),
		Throughput:   b.Throughput(),
	}
}

// FunctionMetrics are the metrics computed per function using all its backends
type FunctionMetrics struct {
	metrics map[*url.URL]*BackendMetrics
	lock    sync.RWMutex
	WindowParameters
}

// NewFunctionMetrics returns a new function metrics
func NewFunctionMetrics(params WindowParameters) *FunctionMetrics {
	return &FunctionMetrics{
		metrics:          make(map[*url.URL]*BackendMetrics),
		lock:             sync.RWMutex{},
		WindowParameters: params,
	}
}

// GetBackend retrieves a specific backend metrics
func (f *FunctionMetrics) GetBackend(b *url.URL) (metrics *BackendMetrics, found bool) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	metrics, found = f.metrics[b]
	return
}

// SyncBackends keeps the backend pool up to date. It adds the new one
// and removes the ones not needed anymore
func (f *FunctionMetrics) SyncBackends(backends []*url.URL) {
	deleteSet := make(map[*url.URL]bool)

	// populate the set with the current pool
	for actualBackend := range f.metrics {
		deleteSet[actualBackend] = true
	}

	// remove and initialize the backends that still serves the functions
	for _, desiredBackend := range backends {
		f.SetBackend(desiredBackend)
		delete(deleteSet, desiredBackend)
	}

	// remove old backends
	for oldBackend := range deleteSet {
		f.RemoveBackend(oldBackend)
	}
}

// SetBackend adds a backend that server a function if it does not already exists
func (f *FunctionMetrics) SetBackend(b *url.URL) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if _, exists := f.metrics[b]; !exists {
		f.metrics[b] = NewBackendMetrics(f.WindowParameters)
	}
}

// RemoveBackend removes a backend from the function pool
func (f *FunctionMetrics) RemoveBackend(b *url.URL) {
	f.lock.Lock()
	defer f.lock.Unlock()
	delete(f.metrics, b)
}

// ResponseTime return the function average response time
func (f *FunctionMetrics) ResponseTime() (responseTime float64) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	var requestCount float64 = 0
	for _, b := range f.metrics {
		rq := b.RequestCount()
		rt := b.ResponseTime()

		if rt != 0 {
			responseTime += rt * rq
			requestCount += rq
		}
	}

	responseTime = responseTime / requestCount

	if math.IsNaN(responseTime) {
		responseTime = 0
	}

	return

}

// RequestCount return the current number of request sent to the function
func (f *FunctionMetrics) RequestCount() (requestCount float64) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	for _, b := range f.metrics {
		requestCount += b.RequestCount()
	}

	if math.IsNaN(requestCount) {
		requestCount = 0
	}
	return
}

// Throughput returns the function throughput in request per second
func (f *FunctionMetrics) Throughput() (throughput float64) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	for _, b := range f.metrics {
		throughput += b.Throughput()
	}

	if math.IsNaN(throughput) {
		throughput = 0
	}

	return

}

// Metrics returns the exposed metrics
func (f *FunctionMetrics) Metrics() ExposedMetrics {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return ExposedMetrics{
		ResponseTime: f.ResponseTime(),
		RequestCount: f.RequestCount(),
		Throughput:   f.Throughput(),
	}
}

// // BackendsMetrics returns the list of backend metrics
// func (f *FunctionMetrics) BackendsMetrics() []*ExposedMetrics {
// 	f.lock.RLock()
// 	defer f.lock.RUnlock()
// 	var metricsList []*ExposedMetrics
// 	for name, backend := range f.metrics {
// 		metrics := backend.metrics()
// 		metrics.Name = name
// 		metricsList = append(metricsList, metrics)
// 	}
// 	return metricsList
// }
