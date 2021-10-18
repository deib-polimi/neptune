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

type PodGetter interface {
	Pods(namespace string) corelisters.PodNamespaceLister
	GetPodsOfFunctionInNode(function *openfaasv1.Function, nodeName string) ([]*corev1.Pod, error)
	GetPodsOfAllFunctionInNode(namespace, nodeName string) ([]*corev1.Pod, error)
}

type NodeGetter interface {
	Nodes() corelisters.NodeLister
}

type FunctionGetter interface {
	Functions(namespace string) openfaaslisters.FunctionNamespaceLister
}

func NewPodGetter(
	pods func(namespace string) corelisters.PodNamespaceLister,
) (PodGetter, error) {

	if pods == nil {
		return nil, fmt.Errorf("pod lister not set in listers struct")
	}

	return &listers{
		pods: pods,
	}, nil
}

type listers struct {
	pods      func(namespace string) corelisters.PodNamespaceLister
	functions func(namespace string) openfaaslisters.FunctionNamespaceLister
	nodes     corelisters.NodeLister
}

func NewListers(
	pods func(namespace string) corelisters.PodNamespaceLister,
	functions func(namespace string) openfaaslisters.FunctionNamespaceLister,
	nodes corelisters.NodeLister,
) *listers {
	return &listers{
		pods:      pods,
		functions: functions,
		nodes:     nodes,
	}
}

func (l listers) Pods(namespace string) corelisters.PodNamespaceLister {
	return l.pods(namespace)
}

func (l listers) Functions(namespace string) openfaaslisters.FunctionNamespaceLister {
	return l.functions(namespace)
}

func (l listers) Nodes() corelisters.NodeLister {
	return l.nodes
}

// GetPodsOfAllFunctionInNode returns a list of pods managed by a community controller
func (l listers) GetPodsOfAllFunctionInNode(namespace, nodeName string) ([]*corev1.Pod, error) {
	selector := labels.SelectorFromSet(
		map[string]string{
			ealabels.NodeLabel: nodeName,
		})
	return l.pods(namespace).List(selector)
}

// GetPodsOfFunctionInNode returns a list of pods which is related to a given function and are running in a given node
func (r listers) GetPodsOfFunctionInNode(function *openfaasv1.Function, nodeName string) ([]*corev1.Pod, error) {
	selector := labels.SelectorFromSet(
		map[string]string{
			ealabels.FunctionNamespaceLabel: function.Namespace,
			ealabels.FunctionNameLabel:      function.Name,
			ealabels.NodeLabel:              nodeName,
		})
	return r.Pods(function.Namespace).List(selector)
}
