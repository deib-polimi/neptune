package controller

import (
	"fmt"
	"net/url"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

func (c *LoadBalancerController) syncCommunitySchedule(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
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
		if source != c.node {
			continue
		}

		for function, destinationRules := range functionRules {
			var lb *balancer.LoadBalancer
			var exist bool

			// create a new load balancer if it does not exist
			if lb, exist = c.balancers[function]; !exist {
				lb = balancer.NewLoadBalancer(c.monitoringChan)
				c.balancers[function] = lb
			}

			actualBackends := []*url.URL{}
			actualBackendsName := []string{}

			for destination, workload := range destinationRules {
				// TODO: use the community controller way to retrieve the function pod running on a node
				// TODO: maybe use Endpoints
				destinationURL, err := url.Parse(destination)

				if err != nil {
					utilruntime.HandleError(fmt.Errorf("error parsing function url: %s", err))
					continue
				}

				// sync load balancer backends with the new rules
				if !lb.ServerExists(destinationURL) {
					lb.AddServer(destinationURL, &workload, c.requestqueue.Enqueue)
				} else {
					lb.UpdateWorkload(destinationURL, &workload)
				}

				actualBackends = append(actualBackends, destinationURL)
				actualBackendsName = append(actualBackendsName, destination)

				// clean old backends
				deleteSet := lb.ServerPoolDiff(actualBackends)

				for _, b := range deleteSet {
					lb.DeleteServer(b)
				}
			}

			c.backendChan <- monitoring.BackendList{
				FunctionURL: function,
				Backends:    actualBackendsName,
			}
		}
	}

	c.recorder.Event(cs, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
