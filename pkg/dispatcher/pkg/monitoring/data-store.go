package monitoring

import (
	"net/url"

	"github.com/lterrac/edge-autoscaler/pkg/metrics"
	"github.com/modern-go/concurrent"
)

// Deprecated: The whole package is deprecated

// BackendList is used to update the backends associated to a function
type BackendList struct {
	// FunctionURL is the URL of the function invo
	FunctionURL string
	// Backends are the backends that serves a function
	Backends []*url.URL
}

// FunctionList is used to keep in sync the function actually serverd by a node
type FunctionList struct {
	// FunctionURL is the URL of the function invo
	Functions []string
}

// DataStore is the main data structure holding all the metrics
type DataStore struct {
	backendChan      <-chan BackendList
	functionChan     <-chan FunctionList
	metricChan       <-chan metrics.RawMetricData
	metrics          *concurrent.Map
	windowParameters metrics.WindowParameters
}

// NewDataStore returns a new DataStore
func NewDataStore(backendChan <-chan BackendList, metricChan <-chan metrics.RawMetricData, functionChan <-chan FunctionList, params metrics.WindowParameters) *DataStore {
	metricMap := &concurrent.Map{}
	// TODO: customize window params with env var or flags?
	return &DataStore{
		backendChan:      backendChan,
		metricChan:       metricChan,
		functionChan:     functionChan,
		metrics:          metricMap,
		windowParameters: params,
	}
}

// Poll will poll the metrics from the channel and update the metrics
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

	var ms *metrics.FunctionMetrics
	var value interface{}

	value, _ = ds.metrics.LoadOrStore(metric.FunctionURL, metrics.NewFunctionMetrics(ds.windowParameters))
	ms = value.(*metrics.FunctionMetrics)

	// ensure that the current backend exists
	ms.SetBackend(metric.Backend)

	backend, _ := ms.GetBackend(metric.Backend)
	backend.AddValue(metric.Value)
}

//TODO: Everything is good if we assume that a balancer serves all possible functions. Otherwise we should implement a cleaning mechanism to remove old functions from the data store

func (ds *DataStore) updateBackends(function string, backends []*url.URL) {
	functionBackends, found := ds.metrics.Load(function)
	if !found {
		return
	}
	functionBackends.(*metrics.FunctionMetrics).SyncBackends(backends)
}
