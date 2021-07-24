package controller

import (
	"fmt"
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetPodsOfFunction returns a list of pods which is related to a given function
func (c *CommunityController) GetPodsOfFunction(function *openfaasv1.Function) ([]*corev1.Pod, error) {
	selector := labels.SelectorFromSet(
		map[string]string{
			"controller":    function.Name,
			"faas_function": function.Spec.Name,
		})
	return c.listers.PodLister.Pods(function.Namespace).List(selector)
}

// GetFunctionOfPod returns the function related to a given pod
func (c *CommunityController) GetFunctionOfPod(pod *corev1.Pod) (*openfaasv1.Function, error) {
	return c.listers.FunctionLister.Functions(pod.Namespace).Get(pod.ObjectMeta.Labels["controller"])
}

// NewCommunitySchedule returns a new empty community schedule with a given namespace and name
func NewCommunitySchedule(namespace, name string) *eav1alpha1.CommunitySchedule {
	return &eav1alpha1.CommunitySchedule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "edgeautoscaler.polimi.it/v1alpha1",
			Kind:       "CommunitySchedule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: name},
		Spec: eav1alpha1.CommunityScheduleSpec{
			RoutingRules: make(eav1alpha1.CommunitySourceRoutingRule),
			Allocations:  make(eav1alpha1.CommunityFunctionAllocation),
		},
	}
}

func (c *CommunityController) getNodeDelays() ([][]int64, error) {

	// Retrieve the nodes
	nodeSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String(): c.communityName,
		})
	nodes, err := c.listers.NodeLister.List(nodeSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve nodes using selector %s with error: %s", nodeSelector, err)
	}

	delays := make([][]int64, len(nodes))
	for i := range delays {
		delays[i] = make([]int64, len(nodes))
	}
	return delays, nil

}

func (c *CommunityController) getWorkload() ([][]int64, error) {

	// Retrieve the nodes
	nodeSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String(): c.communityName,
		})
	nodes, err := c.listers.NodeLister.List(nodeSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve nodes using selector %s with error: %s", nodeSelector, err)
	}

	// Retrieve the functions
	functions, err := c.listers.FunctionLister.Functions(c.communityNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve functions with error: %s", err)
	}

	workloads := make([][]int64, len(nodes))
	for i := range workloads {
		workloads[i] = make([]int64, len(functions))
	}
	return workloads, nil

}

func (c *CommunityController) getMaxDelays() ([]int64, error) {

	// Retrieve the functions
	functions, err := c.listers.FunctionLister.Functions(c.communityNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve functions with error: %s", err)
	}

	delays := make([]int64, len(functions))

	return delays, nil

}
