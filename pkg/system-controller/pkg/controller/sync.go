package controller

import (
	"fmt"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	eaapi "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	slpaclient "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	// EmptyNodeListError is the default error message when grouping cluster nodes
	EmptyNodeListError string = "there are no or too few ready nodes for building communities"
)

func (c *SystemController) syncCommunityConfiguration(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CC resource with this namespace/name
	cc, err := c.listers.CommunityConfigurationLister.Get(name)

	//TODO: handle multiple Community Settings in cluster. Now there should be ONLY one configuration per cluster
	if err != nil {
		// The CC resource may no longer exist, so we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("CommunityConfiguraton '%s' in work queue no longer exists", key))

			klog.Info("Clearing nodes' labels")
			c.communityUpdater.ClearNodes()
			return nil
		}
		return err
	}

	communities, err := c.ComputeCommunities(cc)

	if err != nil {
		return fmt.Errorf("error while executing SLPA: %s", err)
	}

	// update labels on corev1.Node with the corresponding community
	err = c.communityUpdater.UpdateCommunityNodes(communities)

	if err != nil {
		return fmt.Errorf("error while updating nodes: %s", err)
	}

	c.recorder.Event(cc, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// ComputeCommunities divides cluster nodes into communities according to the settings passed as input
// using SLPA
func (c *SystemController) ComputeCommunities(cc *eaapi.CommunityConfiguration) ([]slpaclient.Community, error) {
	// create the input JSON request used by SLPA algorithm
	req, err := c.fetchSLPAData(cc)

	if err != nil {
		return nil, fmt.Errorf("fetching slpa data failed: %s", err)
	}

	// send the request to SLPA and read results
	return c.communityGetter.Communities(req)
}

func (c *SystemController) fetchSLPAData(cc *eaapi.CommunityConfiguration) (*slpaclient.RequestSLPA, error) {

	// Get all nodes in cluster
	nodes, err := c.listers.NodeLister.List(labels.Everything())

	if err != nil {
		return nil, err
	}

	// Keep only the ones in ready state.
	nodes, err = filterReadyNodes(nodes)

	if err != nil {
		return nil, fmt.Errorf("error while filtering ready nodes: %s", err)
	}

	// Get delay matrix
	delays, err := c.getNodeDelays(nodes)

	if err != nil {
		return nil, fmt.Errorf("error while retrieving node delays: %s", err)
	}

	c.communityGetter.SetHost(cc.Spec.SlpaService)

	request := slpaclient.NewRequestSLPA(cc, nodes, delays)

	return request, nil
}

func filterReadyNodes(nodes []*corev1.Node) (result []*corev1.Node, err error) {
	// TODO: once delay discovery is implemented modify it according to changes

	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {

			// don't consider master nodes for building communities
			if _, isMaster := node.Labels[ealabels.MasterNodeLabel]; isMaster {
				continue
			}

			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				result = append(result, node)
			}
		}
	}

	if len(result) < 1 {
		return result, fmt.Errorf(EmptyNodeListError)
	}

	return result, nil
}

// TODO: Up to this point the delay matrix is hard coded
func (c *SystemController) getNodeDelays(nodes []*corev1.Node) (delays [][]int32, err error) {
	delays = make([][]int32, len(nodes))

	for i := range delays {
		delays[i] = make([]int32, len(nodes))
	}

	// TODO: refactor once delay discovery implemented
	err = nil
	for source := range nodes {
		for destination := range nodes {
			value := 2

			if source == destination {
				value = 0
			}
			delays[source][destination] = int32(value)
		}
	}

	return
}
