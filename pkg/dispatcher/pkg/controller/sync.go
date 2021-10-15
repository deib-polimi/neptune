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

	klog.Infof("syncing community schedule %s", key)

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

	klog.Infof("syncing community schedule %s/%s", namespace, name)

	// retrieve routing rules
	sourceRules := cs.Spec.CpuRoutingRules

	for source, functionRules := range sourceRules {
		if source != node.Name {
			continue
		}

		klog.Infof("source node %s\n", source)

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
			klog.Infof("route to function %s\n", functionNamespaceName)

			var lb *balancer.LoadBalancer

			value, _ := c.balancers.LoadOrStore(functionNamespaceName, balancer.NewLoadBalancer(
				balancer.NodeInfo{
					Node:      source,
					Function:  funcName,
					Namespace: funcNamespace,
					Community: community,
				},
				c.metricChan,
			))

			lb = value.(*balancer.LoadBalancer)

			actualBackends := []*url.URL{}
			klog.Infof("backend in routing rules for function: %v in node: %v", functionNamespaceName, source)

			for destination := range destinationRules {
				klog.Infof("destination: %v", destination)
			}

			for destination, workload := range destinationRules {
				pods, err := c.resGetter.GetPodsOfFunctionInNode(function, destination)

				klog.Info("pods of function")

				for _, pod := range pods {
					klog.Infof("pod: %v in node: %v", pod.Name, pod.Spec.NodeName)
				}

				if err != nil {
					utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
					continue
				}

				klog.Info("destination nodes\n")

				for _, pod := range pods {
					// TODO: handle nodes with GPU and CPU
					// TODO: find better way to retrieve function port

					if !isPodReady(pod) {
						continue
					}

					destinationURL, err := url.Parse(fmt.Sprintf("http://%s:%d", pod.Status.PodIP, 8000))

					if err != nil {
						utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
						continue
					}

					// sync load balancer backends with the new weights
					if !lb.ServerExists(destinationURL) {
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

func isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
