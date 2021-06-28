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

// UpdateNodeFunc updates the node in the cluster with the corresponding label
type UpdateNodeFunc func(ctx context.Context, node *corev1.Node, opts v1.UpdateOptions) (*corev1.Node, error)

// ListNodeFunc retrieves the nodes used to build communities
type ListNodeFunc func(selector labels.Selector) (ret []*corev1.Node, err error)

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

			if comm, ok := labels[CommunityLabel]; !ok || comm != community.Name {
				labels[CommunityLabel] = community.Name
			}

			if role, ok := labels[CommunityRoleLabel]; !ok || role != member.Labels[CommunityRoleLabel] {
				labels[CommunityRoleLabel] = member.Labels[CommunityRoleLabel].(string)
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

		if _, ok := labels[CommunityLabel]; !ok {
			continue
		}

		delete(labels, CommunityLabel)

		if _, ok := labels[CommunityRoleLabel]; !ok {
			continue
		}

		delete(labels, CommunityRoleLabel)

		_, err := c.updateFunc(context.TODO(), node, v1.UpdateOptions{})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error while deleting label from node Node %s: %s", node.Name, err))
			return err
		}

		c.clearedNodes[node.Name] = node
	}

	return nil
}
