package apiutils

import (
	"fmt"
	"github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/delayclient"
	"k8s.io/klog/v2"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	openfaaslisters "github.com/openfaas/faas-netes/pkg/client/listers/openfaas/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
)

type ResourceGetter struct {
	pods      func(namespace string) corelisters.PodNamespaceLister
	functions func(namespace string) openfaaslisters.FunctionNamespaceLister
	nodes     corelisters.NodeLister
}

func NewResourceGetter(
	pods func(namespace string) corelisters.PodNamespaceLister,
	functions func(namespace string) openfaaslisters.FunctionNamespaceLister,
	nodes corelisters.NodeLister,
) *ResourceGetter {
	return &ResourceGetter{
		pods:      pods,
		functions: functions,
		nodes:     nodes,
	}
}

// GetPodsOfFunction returns a list of pods which is related to a given function
func (r *ResourceGetter) GetPodsOfFunction(function *openfaasv1.Function) ([]*corev1.Pod, error) {
	selector := labels.SelectorFromSet(
		map[string]string{
			ealabels.FunctionNamespaceLabel: function.Namespace,
			ealabels.FunctionNameLabel:      function.Name,
		})
	return r.pods(function.Namespace).List(selector)
}

// GetFunctionOfPod returns the function related to a given pod
func (r *ResourceGetter) GetFunctionOfPod(pod *corev1.Pod) (*openfaasv1.Function, error) {
	namespace, ok := pod.Labels[ealabels.FunctionNamespaceLabel]
	if !ok {
		return nil, fmt.Errorf("function namespace not found in labels %v", pod.Labels)
	}
	name, ok := pod.Labels[ealabels.FunctionNameLabel]
	if !ok {
		return nil, fmt.Errorf("function name not found in labels %v", pod.Labels)
	}
	return r.functions(namespace).Get(name)
}

// GetPodsOfFunctionInNode returns a list of pods which is related to a given function and are running in a given node
func (r *ResourceGetter) GetPodsOfFunctionInNode(function *openfaasv1.Function, nodeName string) ([]*corev1.Pod, error) {
	selector := labels.SelectorFromSet(
		map[string]string{
			ealabels.FunctionNamespaceLabel: function.Namespace,
			ealabels.FunctionNameLabel:      function.Name,
			ealabels.NodeLabel:              nodeName,
		})
	return r.pods(function.Namespace).List(selector)
}

func (r *ResourceGetter) GetNodeDelays(nodes []string) ([][]int64, error) {
	nodeMapping := make(map[string]int, len(nodes))
	for i, node := range nodes {
		nodeMapping[node] = i
	}

	delayMatrix := make([][]int64, len(nodes))
	for i, _ := range delayMatrix {
		delayMatrix[i] = make([]int64, len(nodes))
	}

	c := delayclient.NewDelayClient(NewDBOptions())
	err := c.SetupDBConnection()
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	delays, err := c.GetDelays()
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	for _, delay := range delays {
		delayMatrix[nodeMapping[delay.FromNode]][nodeMapping[delay.ToNode]] = int64(delay.Latency)
	}

	return delayMatrix, nil
}

func (r *ResourceGetter) GetWorkload(community, communityNamespace string) ([][]int64, error) {

	// Retrieve the nodes
	nodeSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(communityNamespace).String(): community,
		})
	nodes, err := r.nodes.List(nodeSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve nodes using selector %s with error: %s", nodeSelector, err)
	}

	// Retrieve the functions
	functions, err := r.functions(communityNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve functions with error: %s", err)
	}

	workloads := make([][]int64, len(nodes))
	for i := range workloads {
		workloads[i] = make([]int64, len(functions))
	}
	return workloads, nil

}

func (r *ResourceGetter) GetMaxDelays(communityNamespace string) ([]int64, error) {

	// Retrieve the functions
	functions, err := r.functions(communityNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve functions with error: %s", err)
	}

	delays := make([]int64, len(functions))

	return delays, nil

}
