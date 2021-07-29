package controller

import (
	"fmt"
	"net/url"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/queue"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *LoadBalancerController) syncCommunitySchedule(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	node, err := c.listers.NodeLister.Get(c.node)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't find node %s: %v", c.node, err))
		return nil
	}

	nodeLabels := node.GetLabels()

	if communityName, exists := nodeLabels[ealabels.CommunityLabel.WithNamespace(namespace).String()]; !exists || communityName != name {
		return nil
	}

	// Get the CommunitySchedules resource with this namespace/name
	cs, err := c.listers.CommunityScheduleLister.CommunitySchedules(namespace).Get(name)

	if err != nil {
		// The CommunitySchedules may no longer exist, so we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("CommunitySchedules '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// retrieve routing rules
	sourceRules := cs.Spec.RoutingRules

	for source, functionRules := range sourceRules {
		if source != node.Name {
			continue
		}

		for functionNamespaceName, destinationRules := range functionRules {

			funcNamespace, funcName, err := cache.SplitMetaNamespaceKey(functionNamespaceName)

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", functionNamespaceName))
				continue
			}

			function, err := c.listers.FunctionLister.Functions(funcNamespace).Get(funcName)

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("couldn't find function %s: %v", funcName, err))
				continue
			}

			var lb *balancer.LoadBalancer
			var exist bool

			// create a new load balancer if it does not exist
			// TODO: check key
			if lb, exist = c.balancers[functionNamespaceName]; !exist {
				lb = balancer.NewLoadBalancer(c.monitoringChan)
				c.balancers[functionNamespaceName] = lb
			}

			actualBackends := []*url.URL{}

			for destination, workload := range destinationRules {
				pods, err := c.GetPodsOfFunctionInNode(function, destination)

				if err != nil {
					utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
					continue
				}

				for _, pod := range pods {
					// TODO: handle nodes with GPU and CPU
					// TODO: find better way to retrieve function port
					klog.Info(fmt.Sprintf("adding balancer for http://%s:%d", pod.Status.PodIP, pod.Spec.Containers[0].Ports[0].ContainerPort))
					destinationURL, err := url.Parse(fmt.Sprintf("http://%s:%d", pod.Status.PodIP, pod.Spec.Containers[0].Ports[0].ContainerPort))

					if err != nil {
						utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
						continue
					}

					// sync load balancer backends with the new rules
					if !lb.ServerExists(destinationURL) {
						//TODO: remove recovery func
						lb.AddServer(destinationURL, &workload, func(req *queue.HTTPRequest) {})
					} else {
						lb.UpdateWorkload(destinationURL, &workload)
					}

					actualBackends = append(actualBackends, destinationURL)
				}
			}

			// clean old backends
			deleteSet := lb.ServerPoolDiff(actualBackends)

			for _, b := range deleteSet {
				lb.DeleteServer(b)
			}

			c.backendChan <- monitoring.BackendList{
				FunctionURL: functionNamespaceName,
				Backends:    actualBackends,
			}
		}
	}

	c.recorder.Event(cs, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// GetPodsOfFunctionInNode returns a list of pods which is related to a given function and are running in a given node
func (c *LoadBalancerController) GetPodsOfFunctionInNode(function *openfaasv1.Function, nodeName string) ([]*corev1.Pod, error) {
	klog.Infof("controller: %v faas_function: %v ns: %v", function.Name, function.Spec.Name, function.Namespace)
	selector := labels.SelectorFromSet(
		map[string]string{
			"controller":    function.Name,
			"faas_function": function.Spec.Name,
		})
	pods, err := c.listers.PodLister.Pods(function.Namespace).List(selector)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pods using selector %s with error: %s", selector, err)
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
