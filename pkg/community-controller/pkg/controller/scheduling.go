package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sort"
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

type SchedulingInput struct {
	Community           string                                 `json:"community"`
	Namespace           string                                 `json:"namespace"`
	NodeNames           []string                               `json:"node_names"`
	GpuNodeNames        []string                               `json:"gpu_node_names"`
	FunctionNames       []string                               `json:"function_names"`
	GpuFunctionNames    []string                               `json:"gpu_function_names"`
	NodeCores           []int64                                `json:"node_cores"`
	NodeMemories        []int64                                `json:"node_memories"`
	GpuNodeMemories     []int64                                `json:"gpu_node_memories"`
	FunctionMemories    []int64                                `json:"function_memories"`
	GPUFunctionMemories []int64                                `json:"gpu_function_memories"`
	FunctionMaxDelays   []int64                                `json:"function_max_delays"`
	ActualCPUAllocation eav1alpha1.CommunityFunctionAllocation `json:"actual_cpu_allocations"`
	ActualGPUAllocation eav1alpha1.CommunityFunctionAllocation `json:"actual_gpu_allocations"`
}

const (
	NodeCorePadding   = 500
	NodeMemoryPadding = 3000000000
)

type SchedulingOutput struct {
	NodeNames       []string                                 `json:"node_names"`
	FunctionNames   []string                                 `json:"function_names"`
	CpuRoutingRules map[string]map[string]map[string]float64 `json:"cpu_routing_rules"`
	CpuAllocations  map[string]map[string]bool               `json:"cpu_allocations"`
	GpuRoutingRules map[string]map[string]map[string]float64 `json:"gpu_routing_rules"`
	GpuAllocations  map[string]map[string]bool               `json:"gpu_allocations"`
}

type Scheduler struct {
	host       string
	httpClient http.Client
}

// NewSchedulingInput generates a new SchedulingInput
func NewSchedulingInput(
	namespace string,
	community string,
	nodes []*corev1.Node,
	functions []*openfaasv1.Function,
	pods []*corev1.Pod,
	actualCPUAllocation eav1alpha1.CommunityFunctionAllocation,
	actualGPUAllocation eav1alpha1.CommunityFunctionAllocation,
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

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Name < functions[j].Name
	})
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})

	nodeNames := make([]string, nNodes)
	gpuNodeNames := []string{}
	for i, node := range nodes {
		nodeNames[i] = node.Name
		if _, ok := node.Labels[ealabels.GpuNodeLabel]; ok {
			gpuNodeNames = append(gpuNodeNames, node.Name)
		}
	}

	untrackedPods := make(map[string][]*corev1.Pod)
	for _, node := range nodes {
		untrackedPods[node.Name] = make([]*corev1.Pod, 0)
	}

	// Retrieve the pods
	for _, pod := range pods {
		_, hasFunctionName := pod.Labels[ealabels.FunctionNameLabel]
		_, hasFunctionNamespace := pod.Labels[ealabels.FunctionNamespaceLabel]
		if !hasFunctionName && !hasFunctionNamespace {
			if nodePods, ok := untrackedPods[pod.Spec.NodeName]; ok {
				untrackedPods[pod.Spec.NodeName] = append(nodePods, pod)
			}
		}
	}

	nodeCores := make([]int64, nNodes)
	nodeMemories := make([]int64, nNodes)
	gpuNodeMemories := make([]int64, 0)
	for i, node := range nodes {
		nodeCores[i] = node.Status.Capacity.Cpu().MilliValue() - resource.NewMilliQuantity(NodeCorePadding, resource.DecimalSI).MilliValue()
		nodeMemories[i] = node.Status.Capacity.Memory().Value() - resource.NewQuantity(NodeMemoryPadding, resource.DecimalSI).Value()
		if pods, ok := untrackedPods[node.Name]; ok {
			for _, pod := range pods {
				for _, container := range pod.Spec.Containers {
					nodeMemories[i] = nodeMemories[i] - container.Resources.Requests.Memory().Value()
					nodeCores[i] = nodeCores[i] - container.Resources.Requests.Cpu().MilliValue()
				}
			}
		}

		if value, ok := node.Labels[ealabels.GpuNodeMemoryLabel]; ok {
			memory, err := resource.ParseQuantity(value)
			if err != nil {
				klog.Errorf("can not parse value %s as gpu memory (int value) with error %s", value, err)
			}
			gpuNodeMemories = append(gpuNodeMemories, memory.Value())
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
		if _, ok := function.Labels[ealabels.GpuFunctionLabel]; ok {
			gpuFunctionNames = append(gpuFunctionNames, key)
		}
	}

	// For gpu it will not be like this
	functionMemories := make([]int64, nFunctions)
	gpuFunctionMemories := make([]int64, 0)
	for i, function := range functions {
		memoryQuantity, err := resource.ParseQuantity(function.Spec.Requests.Memory)
		if err != nil {
			klog.Errorf("can not parse value %s as memory (int value) with error %s", function.Spec.Requests.Memory, err)
			return nil, err
		}
		memory := memoryQuantity.Value() + HttpMetricsMemory
		functionMemories[i] = memory

		if _, ok := function.Labels[ealabels.GpuFunctionLabel]; ok {

			functionGPUMemory, err := resource.ParseQuantity((*function.Spec.Labels)[ealabels.GpuFunctionMemoryLabel])

			if err != nil {
				klog.Warningf("Function %s gpu memory parsing failed: %v",
					function.Spec.Name, err)
				return nil, err
			}
			gpuFunctionMemories = append(gpuFunctionMemories, functionGPUMemory.Value())
		}
	}

	functionMaxDelays := make([]int64, 0)
	for _, function := range functions {
		value, ok := (*function.Spec.Labels)[ealabels.FunctionMaxDelayLabel]
		if ok {
			maxDelay, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				klog.Errorf("can not parse value %s as gpu max delay (int value) with error %s", value, err)
				return nil, err
			}
			functionMaxDelays = append(functionMaxDelays, maxDelay)
		}
	}

	return &SchedulingInput{
		NodeNames:           nodeNames,
		GpuNodeNames:        gpuNodeNames,
		NodeMemories:        nodeMemories,
		GpuNodeMemories:     gpuNodeMemories,
		FunctionNames:       functionNames,
		GpuFunctionNames:    gpuFunctionNames,
		FunctionMemories:    functionMemories,
		GPUFunctionMemories: gpuFunctionMemories,
		FunctionMaxDelays:   functionMaxDelays,
		NodeCores:           nodeCores,
		ActualCPUAllocation: actualCPUAllocation,
		ActualGPUAllocation: actualGPUAllocation,
		Namespace:           namespace,
		Community:           community,
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

	newCS := cs.DeepCopy()

	newCS.Spec.CpuRoutingRules = so.makeRoutingRules(so.CpuRoutingRules)
	newCS.Spec.GpuRoutingRules = so.makeRoutingRules(so.GpuRoutingRules)

	newCS.Spec.CpuAllocations = so.makeAllocations(so.CpuAllocations)
	newCS.Spec.GpuAllocations = so.makeAllocations(so.GpuAllocations)

	return newCS
}

func (so *SchedulingOutput) makeRoutingRules(actual map[string]map[string]map[string]float64) eav1alpha1.CommunitySourceRoutingRule {
	rr := make(eav1alpha1.CommunitySourceRoutingRule)
	for source, functions := range actual {
		if _, ok := rr[source]; !ok {
			rr[source] = make(eav1alpha1.CommunityFunctionRoutingRule)
		}
		for function, destinations := range functions {
			if _, ok := rr[source][function]; !ok {
				rr[source][function] = make(eav1alpha1.CommunityDestinationRoutingRule)
			}
			for destination, v := range destinations {
				rr[source][function][destination] = *resource.NewMilliQuantity(int64(v*1000), resource.DecimalSI)
			}
		}
	}
	return rr
}

func (so *SchedulingOutput) makeAllocations(actual map[string]map[string]bool) eav1alpha1.CommunityFunctionAllocation {
	allocations := make(eav1alpha1.CommunityFunctionAllocation)
	for function, nodes := range actual {
		if _, ok := allocations[function]; !ok {
			allocations[function] = make(eav1alpha1.CommunityNodeAllocation)
		}
		for node, v := range nodes {
			if v {
				allocations[function][node] = true
			}
		}
	}
	return allocations
}
