package controller

import (
	"context"
	"fmt"

	slpaclient "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func (c *SystemController) updateCommunityNodes(communities []slpaclient.Community) error {
	clusterNodes, err := c.listers.NodeLister.List(labels.Everything())

	if err != nil {
		return err
	}

	nodeMap := make(map[string]*corev1.Node)

	for _, node := range clusterNodes {
		nodeMap[node.Name] = node
	}

	for _, community := range communities {
		for _, member := range community.Members {
			//patch the node with new labels
			node, ok := nodeMap[member.Name]

			if !ok {
				utilruntime.HandleError(fmt.Errorf("Node %s not in node map", member.Name))
				continue
			}

			labels := node.GetLabels()

			if role, ok := labels[CommunityRoleLabel]; !ok || role == member.Labels[CommunityRoleLabel] {
				labels[CommunityRoleLabel] = member.Labels[CommunityRoleLabel].(string)
			}

			_, err := c.kubernetesClientset.CoreV1().Nodes().Update(context.TODO(), node, v1.UpdateOptions{})

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("Error while updating Node %s: %s", member.Name, err))
				return err
			}

			//remove from nodeMap the node up to date
			delete(nodeMap, member.Name)
		}
	}

	// remove nodes that somehow have not been considered for community creation
	for _, node := range nodeMap {
		labels := node.GetLabels()

		if _, ok := labels[CommunityRoleLabel]; !ok {
			continue
		}

		delete(labels, CommunityRoleLabel)

		_, err := c.kubernetesClientset.CoreV1().Nodes().Update(context.TODO(), node, v1.UpdateOptions{})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error while deleting label from node Node %s: %s", node.Name, err))
			return err
		}
	}

	return nil
}
