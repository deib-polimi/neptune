package controller

import (
	"fmt"
	"net/url"

	"github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
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
	cpuRules := cs.Spec.CpuRoutingRules
	gpuRules := cs.Spec.GpuRoutingRules

	c.syncRoutingRules(cpuRules, node.Name, community, false)
	c.syncRoutingRules(gpuRules, node.Name, community, true)

	c.recorder.Event(cs, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *LoadBalancerController) syncRoutingRules(sourceRules v1alpha1.CommunitySourceRoutingRule, nodeName, community string, gpu bool) {

	source := ""
	var functionRules v1alpha1.CommunityFunctionRoutingRule

	for source, functionRules = range sourceRules {
		if source == nodeName {
			break
		}
	}

	if source == "" {
		return
	}

	skipForNotReadyPods := false

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

		for destination, workload := range destinationRules {

			pods, err := c.resGetter.GetPodsOfFunctionInNode(function, destination, gpu)

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
				continue
			}

			if !ArePodsReady(pods) {
				skipForNotReadyPods = true
				break
			}

			for _, pod := range pods {

				destinationURL, err := url.Parse(fmt.Sprintf("http://%s:%d", pod.Status.PodIP, 8000))

				if err != nil {
					utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
					continue
				}

				// sync load balancer backends with the new weights
				if !lb.ServerExists(destinationURL) {
					klog.Infof("load balancer %s add server %s", functionNamespaceName, destination)
					lb.AddServer(destinationURL, destination, gpu, &workload, func(req *queue.HTTPRequest) {})
				} else {
					lb.UpdateWorkload(destinationURL, &workload)
				}

				actualBackends = append(actualBackends, destinationURL)
			}
		}

		if skipForNotReadyPods {
			continue
		}

		// clean old backends
		deleteSet := lb.ServerPoolDiff(actualBackends)

		for _, b := range deleteSet {
			klog.Infof("load balancer %s delete server %s", functionNamespaceName)

			err := lb.DeleteServer(b)

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error deleting server %s: %v", b, err))
			}
		}
	}
}

func ArePodsReady(pods []*corev1.Pod) bool {
	for _, p := range pods {
		if !IsPodReady(p) {
			return false
		}
	}
	return true
}

func IsPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
