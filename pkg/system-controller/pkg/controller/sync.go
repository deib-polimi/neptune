package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	eaapi "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	slpaclient "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

func (c *SystemController) syncCommunitySettings(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CS resource with this namespace/name
	cs, err := c.listers.CommunitySettingses("").Get(name)

	//TODO: handle multiple Community Settings in cluster. Now there should be ONLY one settings per cluster
	if err != nil {
		// The CS resource may no longer exist, so we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("CommunitySettings '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	communities, err := c.ComputeCommunities(cs)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while executing SLPA: %s", err))
		return nil
	}

	// update labels on corev1.Node with the corresponding community
	err = c.communityUpdater.UpdateCommunityNodes(communities)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while updating nodes: %s", err))
		return nil
	}

	c.recorder.Event(cs, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// ComputeCommunities divides cluster nodes into communities according to the settings passed as input
// using SLPA
func (c *SystemController) ComputeCommunities(cs *eaapi.CommunitySettings) ([]slpaclient.Community, error) {
	// create the input JSON request used by SLPA algorithm
	req, err := c.fetchSLPAData(cs)

	if err != nil {
		return nil, fmt.Errorf("fetching slpa data failed: %s", err)
	}

	// send the request to SLPA and read results
	return c.slpaClient.Communities(req)
}

func (c *SystemController) fetchSLPAData(cs *eaapi.CommunitySettings) (*slpaclient.RequestSLPA, error) {

	// Get all nodes in cluster
	nodes, err := c.listers.NodeLister.List(labels.Everything())

	if err != nil {
		return nil, err
	}

	// Keep only the ones in ready state.
	nodes = c.filterReadyNodes(nodes)

	// Get delay matrix
	delays, err := c.getNodeDelays(nodes)

	if err != nil {
		return nil, err
	}

	request := slpaclient.NewRequestSLPA(cs, nodes, delays)

	return request, nil
}

func (c *SystemController) filterReadyNodes(nodes []*corev1.Node) (result []*corev1.Node) {
	// TODO: once delay discovery is implemented modify it according to changes

	for _, node := range nodes {
		conditions := node.Status.Conditions
		var lastStatus corev1.NodeCondition

		for _, condition := range conditions {
			if condition.LastHeartbeatTime.After(lastStatus.LastHeartbeatTime.Time) {
				lastStatus = condition
			}
		}

		if lastStatus.Status == corev1.ConditionTrue {
			result = append(result, node)
		}
	}

	return
}

// TODO: Up to this point the delay matrix is hard coded
func (c *SystemController) getNodeDelays(nodes []*corev1.Node) (delays [][]float32, err error) {
	// TODO: refactor once delay discovery implemented
	err = nil
	for firstNode := range nodes {
		for secondNode := range nodes {
			value := 0.2

			if firstNode == secondNode {
				value = 0
			}
			delays[firstNode][secondNode] = float32(value)
		}
	}

	return
}
