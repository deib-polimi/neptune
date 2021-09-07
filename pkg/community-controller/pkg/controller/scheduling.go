package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"

	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	schedulerServiceURL = "http://localhost:5000/schedule"
)

type SchedulingInput struct {
	NodeNames         []string `json:"node_names"`
	GpuNodeNames      []string `json:"gpu_node_names"`
	FunctionNames     []string `json:"function_names"`
	GpuFunctionNames  []string `json:"gpu_function_names"`
	NodeMemories      []int64  `json:"node_memories"`
	GpuNodeMemories   []int64  `json:"gpu_node_memories"`
	FunctionMemories  []int64  `json:"function_memories"`
	FunctionMaxDelays []int64  `json:"function_max_delays"`
}

type SchedulingOutput struct {
	NodeNames     []string                                 `json:"node_names"`
	FunctionNames []string                                 `json:"function_names"`
	RoutingRules  map[string]map[string]map[string]float64 `json:"routing_rules"`
	Allocations   map[string]map[string]bool               `json:"allocations"`
}

type Scheduler struct {
	host       string
	httpClient http.Client
}

// NewSchedulingInput generates a new SchedulingInput
func NewSchedulingInput(
	nodes []*corev1.Node,
	functions []*openfaasv1.Function,
) (*SchedulingInput, error) {

	// Check input dimensionality
	nNodes := len(nodes)
	nFunctions := len(functions)

	if nNodes <= 0 {
		return nil, fmt.Errorf("number of nodes should be greater than 0")
	}

	if nFunctions <= 0 {
		return nil, fmt.Errorf("number of functions should be greater than 0")
	}

	nodeNames := make([]string, nNodes)
	gpuNodeNames := make([]string, 0)
	for i, node := range nodes {
		nodeNames[i] = node.Name
		_, ok := node.Labels[ealabels.GpuLabel]
		if ok {
			gpuNodeNames = append(gpuNodeNames, node.Name)
		}
	}

	nodeMemories := make([]int64, nNodes)
	gpuNodeMemories := make([]int64, 0)
	for i, node := range nodes {
		nodeMemories[i] = node.Status.Capacity.Memory().MilliValue()
		value, ok := node.Labels[ealabels.GpuMemoryLabel]
		if ok {
			memory, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				klog.Errorf("can not parse value %s as gpu memory (int value) with error %s", value, err)
			}
			gpuNodeMemories = append(gpuNodeMemories, memory)
		}
	}

	functionNames := make([]string, nFunctions)
	gpuFunctionNames := make([]string, 0)
	for i, function := range functions {
		key, err := cache.MetaNamespaceKeyFunc(function)
		if err != nil {
			return nil, err
		}
		functionNames[i] = key
		_, ok := function.Labels[ealabels.GpuFunctionLabel]
		if ok {
			gpuFunctionNames = append(gpuFunctionNames, key)
		}
	}

	functionMemories := make([]int64, nFunctions)
	for i, function := range functions {
		memoryQuantity, err := resource.ParseQuantity(function.Spec.Requests.Memory)
		if err != nil {
			return nil, err
		}
		memory := memoryQuantity.MilliValue()
		functionMemories[i] = memory
	}

	functionMaxDelays := make([]int64, 0)
	for _, function := range functions {
		value, ok := (*function.Spec.Labels)[ealabels.FunctionMaxDelayLabel]
		if ok {
			maxDelay, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				klog.Fatalf("can not parse value %s as gpu max delay (int value) with error %s", value, err)
			}
			functionMaxDelays = append(functionMaxDelays, maxDelay)
		}
	}

	return &SchedulingInput{
		NodeNames:         nodeNames,
		GpuNodeNames:      gpuNodeNames,
		NodeMemories:      nodeMemories,
		GpuNodeMemories:   gpuNodeMemories,
		FunctionNames:     functionNames,
		GpuFunctionNames:  gpuFunctionNames,
		FunctionMemories:  functionMemories,
		FunctionMaxDelays: functionMaxDelays,
	}, nil
}

// NewScheduler returns a new MetricClient representing a metric client.
func NewScheduler(host string) *Scheduler {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 90 * time.Second,
			}).DialContext,
			// TODO: Some of those value should be tuned
			MaxIdleConns:          50,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
		},
		Timeout: 20 * time.Second,
	}
	scheduler := &Scheduler{
		httpClient: httpClient,
		host:       host,
	}
	return scheduler
}

// Schedule runs the scheduling algorithm on the input and returns a schedule to be applied
func (s *Scheduler) Schedule(input *SchedulingInput) (*SchedulingOutput, error) {

	// Json serialize the input
	reqBody, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	// Create request
	request, err := http.NewRequest(http.MethodGet, s.host, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	// Send the request
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	// Populate the scheduling output
	var output SchedulingOutput
	err = json.Unmarshal(resBody, &output)
	if err != nil {
		return nil, err
	}

	return &output, nil

}

// ToCommunitySchedule transform a scheduling output to a Community schedule CRD
func (so *SchedulingOutput) ToCommunitySchedule(cs *eav1alpha1.CommunitySchedule) *eav1alpha1.CommunitySchedule {
	routingRules := make(eav1alpha1.CommunitySourceRoutingRule)
	for source, functions := range so.RoutingRules {
		if _, ok := routingRules[source]; !ok {
			routingRules[source] = make(eav1alpha1.CommunityFunctionRoutingRule)
		}
		for function, destinations := range functions {
			if _, ok := routingRules[source][function]; !ok {
				routingRules[source][function] = make(eav1alpha1.CommunityDestinationRoutingRule)
			}
			for destination, v := range destinations {
				routingRules[source][function][destination] = *resource.NewMilliQuantity(int64(v*1000), resource.DecimalSI)
			}
		}
	}

	allocations := make(eav1alpha1.CommunityFunctionAllocation)
	for function, nodes := range so.Allocations {
		if _, ok := allocations[function]; !ok {
			allocations[function] = make(eav1alpha1.CommunityNodeAllocation)
		}
		for node, v := range nodes {
			if v {
				allocations[function][node] = true
			}
		}
	}

	newCS := cs.DeepCopy()
	newCS.Spec.RoutingRules = routingRules
	newCS.Spec.Allocations = allocations

	return newCS
}
