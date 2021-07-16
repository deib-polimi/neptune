package metrics

import (
	"math"
	"sync"
	"time"

	"github.com/asecurityteam/rolling"
)

// RawMetricData represents the single data point collected by each load balancer
type RawMetricData struct {
	// Backend is the pod which has processed the request
	Backend string
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

// FunctionMetrics are the metrics computed per function using all its backends
type FunctionMetrics struct {
	metrics map[string]*BackendMetrics
	lock    sync.RWMutex
	WindowParameters
}

func NewFunctionMetrics(params WindowParameters) *FunctionMetrics {
	return &FunctionMetrics{
		metrics:          make(map[string]*BackendMetrics),
		lock:             sync.RWMutex{},
		WindowParameters: params,
	}
}

// GetBackend retrieves a specific backend metrics
func (f *FunctionMetrics) GetBackend(b string) (metrics *BackendMetrics, found bool) {
	metrics, found = f.metrics[b]
	return
}

// SetBackend adds a backend that server a function if it does not already exists
func (f *FunctionMetrics) SetBackend(b string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if _, exists := f.metrics[b]; !exists {
		f.metrics[b] = NewBackendMetrics(f.WindowParameters)
	}
}

// RemoveBackend removes a backend from the function pool
func (f *FunctionMetrics) RemoveBackend(b string) {
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
