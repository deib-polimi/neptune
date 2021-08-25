package apiutils

import (
	"fmt"

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
			"controller":    function.Name,
			"faas_function": function.Spec.Name,
		})
	return r.pods(function.Namespace).List(selector)
}

// GetFunctionOfPod returns the function related to a given pod
func (r *ResourceGetter) GetFunctionOfPod(pod *corev1.Pod) (*openfaasv1.Function, error) {
	return r.functions(pod.Namespace).Get(pod.ObjectMeta.Labels["controller"])
}

// GetPodsOfFunctionInNode returns a list of pods which is related to a given function and are running in a given node
func (r *ResourceGetter) GetPodsOfFunctionInNode(function *openfaasv1.Function, nodeName string) ([]*corev1.Pod, error) {
	pods, err := r.GetPodsOfFunction(function)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pods: %s", err)
	}

	var podsInNode []*corev1.Pod
	for _, pod := range pods {
		if pod.Spec.NodeName == nodeName {
			podsInNode = append(podsInNode, pod)
		}
	}

	if len(podsInNode) == 0 {
		return nil, fmt.Errorf("no pods found in node %s", nodeName)
	}

	return podsInNode, nil
}

func (r *ResourceGetter) GetNodeDelays(community, communityNamespace string) ([][]int64, error) {

	// Retrieve the nodes
	nodeSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(communityNamespace).String(): community,
		})
	nodes, err := r.nodes.List(nodeSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve nodes using selector %s with error: %s", nodeSelector, err)
	}

	delays := make([][]int64, len(nodes))
	for i := range delays {
		delays[i] = make([]int64, len(nodes))
	}
	return delays, nil

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
