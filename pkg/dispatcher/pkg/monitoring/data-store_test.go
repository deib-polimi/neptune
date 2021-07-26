package monitoring

import (
	"net/url"
	"testing"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	"github.com/stretchr/testify/require"
)

func TestHandleRawData(t *testing.T) {
	testcases := []struct {
		description string
		sendData    func(c chan<- metrics.RawMetricData, m []metrics.RawMetricData)
		input       []metrics.RawMetricData
		expected    float64
	}{
		{
			description: "one backend per function",
			input: []metrics.RawMetricData{
				{
					Backend:     &url.URL{Host: "foo"},
					FunctionURL: "function-1",
					Value:       1.0,
				},
				{
					Backend:     &url.URL{Host: "foo"},
					FunctionURL: "function-1",
					Value:       6.0,
				},
				{
					Backend:     &url.URL{Host: "foo"},
					FunctionURL: "function-1",
					Value:       8.0,
				},
			},
			sendData: func(c chan<- metrics.RawMetricData, metrics []metrics.RawMetricData) {
				for _, m := range metrics {
					c <- m
				}
			},
			expected: 5.0,
		},
		{
			description: "two backend per function",
			input: []metrics.RawMetricData{
				{
					Backend:     &url.URL{Host: "foo"},
					FunctionURL: "function-1",
					Value:       1.0,
				},
				{
					Backend:     &url.URL{Host: "bar"},
					FunctionURL: "function-1",
					Value:       6.0,
				},
				{
					Backend:     &url.URL{Host: "foo"},
					FunctionURL: "function-1",
					Value:       8.0,
				},
			},
			sendData: func(c chan<- metrics.RawMetricData, metrics []metrics.RawMetricData) {
				for _, m := range metrics {
					time.Sleep(50 * time.Millisecond)
					c <- m
				}
			},
			expected: 5.0,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			backendChan := make(chan BackendList)
			monitoringChan := make(chan metrics.RawMetricData)
			ds := NewDataStore(backendChan, monitoringChan, metrics.WindowParameters{
				WindowSize:        1 * time.Second,
				WindowGranularity: 1 * time.Millisecond,
			})

			ds.Poll()

			tt.sendData(monitoringChan, tt.input)

			actual, _ := ds.metrics.Load("function-1")
			// we are interested in checking if data have been correctly added
			// so there's no need to check for all the other metrics
			require.Equal(t, tt.expected, actual.(*metrics.FunctionMetrics).ResponseTime())
		})
	}
}
