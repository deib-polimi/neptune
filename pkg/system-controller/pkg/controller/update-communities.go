package controller

import (
	"context"
	"fmt"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/labels"

	slpaclient "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// UpdateNodeFunc updates the node in the cluster with the corresponding label
type UpdateNodeFunc func(ctx context.Context, node *corev1.Node, opts v1.UpdateOptions) (*corev1.Node, error)

// ListNodeFunc retrieves the nodes used to build communities
type ListNodeFunc func(selector labels.Selector) (ret []*corev1.Node, err error)

// CommunityUpdater takes care of keeping updated Kubernetes Nodes according to the last SLPA execution
type CommunityUpdater struct {
	updatedNodes map[string]*corev1.Node
	clearedNodes map[string]*corev1.Node
	updateFunc   UpdateNodeFunc
	listFunc     ListNodeFunc
}

// NewCommunityUpdater returns a new CommunityUpdater
func NewCommunityUpdater(updateFunc UpdateNodeFunc, listFunc ListNodeFunc) *CommunityUpdater {
	return &CommunityUpdater{
		updatedNodes: make(map[string]*corev1.Node),
		clearedNodes: make(map[string]*corev1.Node),
		updateFunc:   updateFunc,
		listFunc:     listFunc,
	}
}

// UpdateCommunityNodes applies the new labels to Kubernetes Nodes
func (c *CommunityUpdater) UpdateCommunityNodes(communities []slpaclient.Community) error {
	clusterNodes, err := c.listFunc(labels.Everything())

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

			if comm, ok := labels[ealabels.CommunityLabel]; !ok || comm != community.Name {
				labels[ealabels.CommunityLabel] = community.Name
			}

			if role, ok := labels[ealabels.CommunityRoleLabel.String()]; !ok || role != member.Labels[ealabels.CommunityRoleLabel.String()] {
				labels[ealabels.CommunityRoleLabel.String()] = member.Labels[ealabels.CommunityRoleLabel.String()].(string)
			}

			_, err := c.updateFunc(context.TODO(), node, v1.UpdateOptions{})

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("Error while updating Node %s: %s", member.Name, err))
				return err
			}

			c.updatedNodes[node.Name] = node

			//remove from nodeMap the node up to date
			delete(nodeMap, member.Name)
		}
	}

	// remove nodes that somehow have not been considered for community creation
	for _, node := range nodeMap {
		labels := node.GetLabels()

		if _, ok := labels[ealabels.CommunityLabel]; !ok {
			continue
		}

		delete(labels, ealabels.CommunityLabel)

		if _, ok := labels[ealabels.CommunityRoleLabel.String()]; !ok {
			continue
		}

		delete(labels, ealabels.CommunityRoleLabel.String())

		_, err := c.updateFunc(context.TODO(), node, v1.UpdateOptions{})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error while deleting label from node Node %s: %s", node.Name, err))
			return err
		}

		c.clearedNodes[node.Name] = node
	}

	return nil
}

// ClearNodes removes the labels from nodes
func (c *CommunityUpdater) ClearNodes() error {
	clusterNodes, err := c.listFunc(labels.Everything())

	if err != nil {
		return err
	}

	for _, node := range clusterNodes {
		if _, ok := node.Labels[ealabels.CommunityLabel]; ok {
			delete(node.Labels, ealabels.CommunityLabel)
		}

		if _, ok := node.Labels[ealabels.CommunityRoleLabel.String()]; ok {
			delete(node.Labels, ealabels.CommunityRoleLabel.String())
		}

		_, err = c.updateFunc(context.TODO(), node, v1.UpdateOptions{})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error while deleting label from node Node %s: %s", node.Name, err))
			return err
		}
	}

	return nil
}
