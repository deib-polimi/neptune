package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/cache"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	schedulerServiceURL = "http://localhost:5000/schedule"
)

type SchedulingInput struct {
	NodeNames        []string  `json:"node_names"`
	FunctionNames    []string  `json:"function_names"`
	NodeMemories     []int64   `json:"node_memories"`
	FunctionMemories []int64   `json:"function_memories"`
	Delays           [][]int64 `json:"delay_matrix"`
	MaxDelays        []int64   `json:"max_delays"`
	Workloads        [][]int64 `json:"workload_matrix"`
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
	delays [][]int64,
	workloads [][]int64,
	maxDelays []int64,
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

	if len(delays) != nNodes {
		return nil, fmt.Errorf("delay matrix must be a square matrix with %vx%v elements", nNodes, nNodes)
	}
	for _, a := range delays {
		if len(a) != nNodes {
			return nil, fmt.Errorf("delay matrix must be a square matrix with %vx%v elements", nNodes, nNodes)
		}
	}

	if len(workloads) != nNodes {
		return nil, fmt.Errorf("workload matrix must be a square matrix with %vx%v elements", nNodes, nFunctions)
	}
	for _, a := range workloads {
		if len(a) != nFunctions {
			return nil, fmt.Errorf("workload matrix must be a square matrix with %vx%v elements", nNodes, nFunctions)
		}
	}

	if len(maxDelays) != nFunctions {
		return nil, fmt.Errorf("max delay array must have size %v", nFunctions)
	}

	nodeNames := make([]string, nNodes)
	for i, node := range nodes {
		nodeNames[i] = node.Name
	}

	nodeMemories := make([]int64, nNodes)
	for i, node := range nodes {
		nodeMemories[i] = node.Status.Capacity.Memory().MilliValue()
	}

	functionNames := make([]string, nFunctions)
	for i, function := range functions {
		key, err := cache.MetaNamespaceKeyFunc(function)
		if err != nil {
			return nil, err
		}
		functionNames[i] = key
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

	return &SchedulingInput{
		NodeNames:        nodeNames,
		NodeMemories:     nodeMemories,
		Delays:           delays,
		FunctionNames:    functionNames,
		FunctionMemories: functionMemories,
		Workloads:        workloads,
		MaxDelays:        maxDelays,
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

// Apply applies a scheduling output
func (s *Scheduler) Apply(communityNamespace, communityName string, output *SchedulingOutput, function *openfaasv1.Function, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {

	key, err := cache.MetaNamespaceKeyFunc(function)
	if err != nil {
		return nil, err
	}

	instances := len(output.Allocations[key])

	if deployment.Labels == nil {
		deployment.Labels = make(map[string]string)
	}

	deployment.Labels[CommunityInstancesLabel.WithNamespace(communityNamespace).WithName(communityName).String()] = strconv.Itoa(instances)

	return deployment, nil

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
