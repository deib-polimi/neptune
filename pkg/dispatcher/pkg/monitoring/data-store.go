package monitoring

import (
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	"github.com/modern-go/concurrent"
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
	metrics          concurrent.Map
	windowParameters metrics.WindowParameters
	exposer          *Exposer
}

// NewDataStore returns a new DataStore
func NewDataStore(backendChan <-chan BackendList, metricChan <-chan metrics.RawMetricData, params metrics.WindowParameters) *DataStore {
	metricMap := concurrent.Map{}
	return &DataStore{
		backendChan: backendChan,
		metricChan:  metricChan,
		metrics:     metricMap,
		// TODO: customize with env var?
		windowParameters: params,
		exposer:          NewExposer(metricMap),
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

// Expose exposes the metrics
func (ds *DataStore) Expose() {
	ds.exposer.Run()
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

func (ds *DataStore) updateBackends(function string, backends []string) {
	functionBackends, found := ds.metrics.Load(function)
	if !found {
		return
	}
	functionBackends.(*metrics.FunctionMetrics).SyncBackends(backends)
}
