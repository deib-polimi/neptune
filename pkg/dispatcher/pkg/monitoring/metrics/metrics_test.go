package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackendMetrics(t *testing.T) {
	testcases := []struct {
		description  string
		input        []float64
		testFunction func(b *BackendMetrics) float64
		desired      float64
	}{
		{
			description: "test response time",
			input:       []float64{1.0, 2.0, 3.0},
			testFunction: func(b *BackendMetrics) float64 {
				return b.ResponseTime()
			},
			desired: 2.0,
		},
		{
			description: "test empty response time",
			input:       []float64{},
			testFunction: func(b *BackendMetrics) float64 {
				return b.ResponseTime()
			},
			desired: 0.0,
		},
		{
			description: "test request count",
			input:       []float64{1.0, 2.0, 3.0},
			testFunction: func(b *BackendMetrics) float64 {
				return b.RequestCount()
			},
			desired: 3.0,
		},
		{
			description: "test empty request count",
			input:       []float64{},
			testFunction: func(b *BackendMetrics) float64 {
				return b.RequestCount()
			},
			desired: 0.0,
		},
		{
			description: "test throughput",
			input:       []float64{1.0, 2.0, 3.0},
			testFunction: func(b *BackendMetrics) float64 {
				return b.Throughput()
			},
			desired: 3.0,
		},
		{
			description: "test empty throughput",
			input:       []float64{},
			testFunction: func(b *BackendMetrics) float64 {
				return b.Throughput()
			},
			desired: 0.0,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			b := NewBackendMetrics(WindowParameters{
				WindowSize:        1 * time.Second,
				WindowGranularity: 1 * time.Millisecond,
			})

			for _, data := range tt.input {
				time.Sleep(15 * time.Millisecond)
				b.window.Append(data)
			}

			actual := tt.testFunction(b)
			require.Equal(t, tt.desired, actual)
		})
	}
}

func TestFunctionMetrics(t *testing.T) {
	testcases := []struct {
		description  string
		input        map[string][]float64
		testFunction func(f *FunctionMetrics) float64
		desired      float64
	}{
		{
			description: "test response time",
			input: map[string][]float64{
				"foo":    {1.0, 2.0, 3.0},
				"bar":    {3.0, 4.0, 5.0},
				"foobar": {5.0, 6.0, 7.0},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.ResponseTime()
			},
			desired: 4.0,
		},
		{
			description: "test one empty response time",
			input: map[string][]float64{
				"foo":    {1.0, 2.0, 3.0},
				"bar":    {2.0},
				"foobar": {},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.ResponseTime()
			},
			desired: 2.0,
		},
		{
			description: "test all empty response time",
			input: map[string][]float64{
				"foo":    {},
				"bar":    {},
				"foobar": {},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.ResponseTime()
			},
			desired: 0.0,
		},
		{
			description: "test request count",
			input: map[string][]float64{
				"foo":    {1.0, 2.0, 3.0},
				"bar":    {3.0, 4.0, 5.0},
				"foobar": {5.0, 6.0, 7.0},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.RequestCount()
			},
			desired: 9.0,
		},
		{
			description: "test one empty request count",
			input: map[string][]float64{
				"foo":    {1.0, 2.0, 3.0},
				"bar":    {3.0, 4.0, 5.0},
				"foobar": {},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.RequestCount()
			},
			desired: 6.0,
		},
		{
			description: "test all empty request count",
			input: map[string][]float64{
				"foo":    {},
				"bar":    {},
				"foobar": {},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.RequestCount()
			},
			desired: 0.0,
		},
		{
			description: "test throughput",
			input: map[string][]float64{
				"foo":    {1.0, 2.0, 3.0},
				"bar":    {3.0, 4.0, 5.0},
				"foobar": {5.0, 6.0, 7.0},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.Throughput()
			},
			desired: 9.0,
		},
		{
			description: "test one empty throughput",
			input: map[string][]float64{
				"foo":    {1.0, 2.0, 3.0},
				"bar":    {3.0, 4.0, 5.0},
				"foobar": {},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.Throughput()
			},
			desired: 6.0,
		},
		{
			description: "test all empty throughput",
			input: map[string][]float64{
				"foo":    {},
				"bar":    {},
				"foobar": {},
			},
			testFunction: func(f *FunctionMetrics) float64 {
				return f.Throughput()
			},
			desired: 0.0,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			f := NewFunctionMetrics(WindowParameters{
				WindowSize:        1 * time.Second,
				WindowGranularity: 1 * time.Millisecond,
			})

			for b, data := range tt.input {
				f.SetBackend(b)
				backend, _ := f.GetBackend(b)

				for _, point := range data {
					backend.window.Append(point)
					time.Sleep(1 * time.Millisecond)
				}
			}

			actual := tt.testFunction(f)
			require.Equal(t, tt.desired, actual)
		})
	}
}
