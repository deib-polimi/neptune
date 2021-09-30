package apiutils

import (
	"fmt"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaaslisters "github.com/openfaas/faas-netes/pkg/client/listers/openfaas/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
)

type PodGetter interface {
	Pods(namespace string) corelisters.PodNamespaceLister
	GetPodsOfAllFunctionInNode(namespace, nodeName string) ([]*corev1.Pod, error)
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

// GetPodsOfAllFunctionInNode returns a list of pods managed by a community controller
func (l listers) GetPodsOfAllFunctionInNode(namespace, nodeName string) ([]*corev1.Pod, error) {
	selector := labels.SelectorFromSet(
		map[string]string{
			ealabels.NodeLabel: nodeName,
		})
	return l.pods(namespace).List(selector)
}
