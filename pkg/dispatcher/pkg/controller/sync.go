package controller

import (
	"fmt"
	"net/url"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/queue"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	var community string
	var exists bool

	if community, exists = nodeLabels[ealabels.CommunityLabel.WithNamespace(namespace).String()]; !exists || community != name {
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

		functions := []string{}

		for functionNamespaceName, destinationRules := range functionRules {

			functions = append(functions, functionNamespaceName)
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
				// lb = balancer.NewLoadBalancer(c.monitoringChan)

				lb = balancer.NewLoadBalancer(
					balancer.NodeInfo{
						Node:      source,
						Function:  funcName,
						Namespace: funcNamespace,
						Community: community,
					},
					c.metricChan,
				)
				c.balancers[functionNamespaceName] = lb
			}

			actualBackends := []*url.URL{}

			for destination, workload := range destinationRules {
				pods, err := c.resGetter.GetPodsOfFunctionInNode(function, destination)

				if err != nil {
					utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
					continue
				}

				for _, pod := range pods {
					// TODO: handle nodes with GPU and CPU
					// TODO: find better way to retrieve function port
					klog.Info(fmt.Sprintf("adding balancer for http://%s:%d", pod.Status.PodIP, pod.Spec.Containers[0].Ports[0].ContainerPort))
					destinationURL, err := url.Parse(fmt.Sprintf("http://%s:%d", pod.Status.PodIP, pod.Spec.Containers[0].Ports[0].ContainerPort))
					destinationURL.Scheme = "http"

					if err != nil {
						utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
						continue
					}

					// sync load balancer backends with the new rules
					if !lb.ServerExists(destinationURL) {
						//TODO: remove recovery func
						// TODO: add mechanism to detect gpu instead of a hardcoded false bool
						lb.AddServer(destinationURL, destination, false, &workload, func(req *queue.HTTPRequest) {})
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

			// c.backendChan <- monitoring.BackendList{
			// 	FunctionURL: functionNamespaceName,
			// 	Backends:    actualBackends,
			// }
		}

		// c.functionChan <- monitoring.FunctionList{
		// 	Functions: functions,
		// }
	}

	c.recorder.Event(cs, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
