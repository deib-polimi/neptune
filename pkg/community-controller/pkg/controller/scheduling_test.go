package controller

import (
	"encoding/json"
	"fmt"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"net/http"
	"strconv"
	"testing"
)

func TestNewSchedulingInput(t *testing.T) {
	nNodes := 100
	nFunctions := 500

	testcases := []struct {
		description string
		nNodes      int
		nFunctions  int
		xDelays     int
		yDelays     int
		xWorkload   int
		yWorkload   int
		nMaxDelays  int
		expectError bool
	}{
		{
			description: "standard scenario",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes,
			yDelays:     nNodes,
			xWorkload:   nNodes,
			yWorkload:   nFunctions,
			nMaxDelays:  nFunctions,
			expectError: false,
		},
		{
			description: "no nodes scenario",
			nNodes:      0,
			nFunctions:  nFunctions,
			xDelays:     0,
			yDelays:     0,
			xWorkload:   0,
			yWorkload:   nFunctions,
			nMaxDelays:  nFunctions,
			expectError: true,
		},
		{
			description: "no functions scenario",
			nNodes:      nNodes,
			nFunctions:  0,
			xDelays:     nNodes,
			yDelays:     nNodes,
			xWorkload:   nNodes,
			yWorkload:   0,
			nMaxDelays:  0,
			expectError: true,
		},
		{
			description: "delay matrix mismatch scenario x-axis",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes - 1,
			yDelays:     nNodes,
			xWorkload:   nNodes,
			yWorkload:   nFunctions,
			nMaxDelays:  nFunctions,
			expectError: true,
		},
		{
			description: "delay matrix mismatch scenario y-axis",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes,
			yDelays:     nNodes - 1,
			xWorkload:   nNodes,
			yWorkload:   nFunctions,
			nMaxDelays:  nFunctions,
			expectError: true,
		},
		{
			description: "delay matrix mismatch scenario both axis",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes - 1,
			yDelays:     nNodes - 1,
			xWorkload:   nNodes,
			yWorkload:   nFunctions,
			nMaxDelays:  nFunctions,
			expectError: true,
		},
		{
			description: "workload mismatch scenario x-axis",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes,
			yDelays:     nNodes,
			xWorkload:   nNodes - 1,
			yWorkload:   nFunctions,
			nMaxDelays:  nFunctions,
			expectError: true,
		},
		{
			description: "workload mismatch scenario y-axis",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes,
			yDelays:     nNodes,
			xWorkload:   nNodes,
			yWorkload:   nFunctions - 1,
			nMaxDelays:  nFunctions,
			expectError: true,
		},
		{
			description: "workload mismatch scenario both axis",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes,
			yDelays:     nNodes,
			xWorkload:   nNodes - 1,
			yWorkload:   nFunctions - 1,
			nMaxDelays:  nFunctions,
			expectError: true,
		},
		{
			description: "max delays mismatch scenario",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes,
			yDelays:     nNodes,
			xWorkload:   nNodes,
			yWorkload:   nFunctions,
			nMaxDelays:  nFunctions - 1,
			expectError: true,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {

			nodes := make([]*corev1.Node, tt.nNodes)
			for i := 0; i < tt.nNodes; i++ {
				nodes[i] = newRandomFakeNode(i)
			}

			functions := make([]*openfaasv1.Function, tt.nFunctions)
			for i := 0; i < tt.nFunctions; i++ {
				functions[i] = newRandomFakeFunction(i)
			}

			delays := newIntMatrix(tt.xDelays, tt.yDelays)
			workload := newIntMatrix(tt.xWorkload, tt.yWorkload)
			maxDelays := newIntArray(tt.nMaxDelays)

			result, err := NewSchedulingInput(nodes, functions, delays, workload, maxDelays)
			if err != nil {
				require.True(t, tt.expectError)
			} else {
				require.Equal(t, len(result.NodeNames), tt.nNodes)
				require.Equal(t, len(result.NodeMemories), tt.nNodes)
				require.Equal(t, len(result.FunctionMemories), tt.nFunctions)
				require.Equal(t, len(result.FunctionNames), tt.nFunctions)
				require.False(t, tt.expectError)
			}
		})
	}
}

func TestSchedule(t *testing.T) {

	nNodes := 10
	nFunctions := 50

	testcases := []struct {
		description string
		nNodes      int
		nFunctions  int
		xDelays     int
		yDelays     int
		xWorkload   int
		yWorkload   int
		nMaxDelays  int
		expectError bool
	}{
		{
			description: "standard scenario",
			nNodes:      nNodes,
			nFunctions:  nFunctions,
			xDelays:     nNodes,
			yDelays:     nNodes,
			xWorkload:   nNodes,
			yWorkload:   nFunctions,
			nMaxDelays:  nFunctions,
			expectError: false,
		},
	}

	newFakeSchedulerServer()

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {

			nodes := make([]*corev1.Node, tt.nNodes)
			for i := 0; i < tt.nNodes; i++ {
				nodes[i] = newRandomFakeNode(i)
			}

			functions := make([]*openfaasv1.Function, tt.nFunctions)
			for i := 0; i < tt.nFunctions; i++ {
				functions[i] = newRandomFakeFunction(i)
			}

			delays := newIntMatrix(tt.xDelays, tt.yDelays)
			workload := newIntMatrix(tt.xWorkload, tt.yWorkload)
			maxDelays := newIntArray(tt.nMaxDelays)

			result, err := NewSchedulingInput(nodes, functions, delays, workload, maxDelays)

			if err != nil {
				require.True(t, tt.expectError)
			}

			s := NewScheduler("http://localhost:8080/")
			output, err := s.Schedule(result)

			if err != nil {
				require.True(t, tt.expectError)
			} else {
				require.Equal(t, len(output.NodeNames), tt.nNodes)
				require.Equal(t, len(output.FunctionNames), tt.nFunctions)
				require.False(t, tt.expectError)
			}

		})
	}
}

func newRandomFakeNode(randomSeed int) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name: "Node " + strconv.Itoa(randomSeed),
		},
		Status: corev1.NodeStatus{
			Capacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(int64(randomSeed*3), resource.Milli),
				corev1.ResourceMemory: *resource.NewScaledQuantity(int64(randomSeed*3), resource.Mega),
			},
		},
	}
}

func newFakeSchedulerServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var input SchedulingInput

		// Try to decode the request body into the struct. If there is an error,
		// respond to the client with the error message and a 400 status code.
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Create routing rules
		routingRules := make(map[string]map[string]map[string]float64)
		allocations := make(map[string]map[string]bool)

		for _, source := range input.NodeNames {
			_, ok := routingRules[source]
			if !ok {
				routingRules[source] = make(map[string]map[string]float64)
				for _, dest := range input.NodeNames {
					_, ok = routingRules[source]
					if !ok {
						routingRules[source][dest] = make(map[string]float64)
						for _, f := range input.FunctionNames {
							routingRules[source][dest][f] = 0.0
						}
					}
				}
			}
		}

		for _, node := range input.NodeNames {
			_, ok := allocations[node]
			if !ok {
				allocations[node] = make(map[string]bool)
				for _, f := range input.FunctionNames {
					allocations[node][f] = true
				}
			}
		}

		output := &SchedulingOutput{
			NodeNames:     input.NodeNames,
			FunctionNames: input.FunctionNames,
			RoutingRules:  routingRules,
			Allocations:   allocations,
		}

		resp, err := json.Marshal(output)
		if err != nil {
			klog.Error(err)
		}
		if _, err := w.Write(resp); err != nil {
			klog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	})
	go func() {
		klog.Fatal(http.ListenAndServe(":8080", nil))
	}()
}

func newRandomFakeFunction(randomSeed int) *openfaasv1.Function {
	return &openfaasv1.Function{
		ObjectMeta: v1.ObjectMeta{
			Name:      strconv.Itoa(randomSeed),
			Namespace: strconv.Itoa(randomSeed),
		},
		Spec: openfaasv1.FunctionSpec{
			Limits: &openfaasv1.FunctionResources{
				Memory: strconv.Itoa(randomSeed),
				CPU:    strconv.Itoa(randomSeed),
			},
			Requests: &openfaasv1.FunctionResources{
				Memory: strconv.Itoa(randomSeed),
				CPU:    strconv.Itoa(randomSeed),
			},
		},
	}
}

func newIntMatrix(width int, height int) [][]int64 {
	a := make([][]int64, width)
	for i := range a {
		a[i] = make([]int64, height)
	}
	return a
}

func newIntArray(n int) []int64 {
	return make([]int64, n)
}
