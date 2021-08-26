package controller

import (
	"context"
	"fmt"

	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"

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

// UpdateStatusFunc updates the cc status with the new communities
type UpdateStatusFunc func(ctx context.Context, cc *eav1alpha1.CommunityConfiguration, opts v1.UpdateOptions) (*eav1alpha1.CommunityConfiguration, error)

// UpdateStatusNamespaceGenerator generates an UpdateStatusFunc for a given namespace
type UpdateStatusNamespaceGenerator func(ns string) UpdateStatusFunc

// CommunityUpdater takes care of keeping updated Kubernetes Nodes according to the last SLPA execution
type CommunityUpdater struct {
	updatedNodes                   map[string]*corev1.Node
	clearedNodes                   map[string]*corev1.Node
	updateNode                     UpdateNodeFunc
	listNodes                      ListNodeFunc
	updateStatusNamespaceGenerator UpdateStatusNamespaceGenerator
}

// NewCommunityUpdater returns a new CommunityUpdater
func NewCommunityUpdater(updateNodeFunc UpdateNodeFunc, listNodesFunc ListNodeFunc, eaclient eaclientset.Interface) *CommunityUpdater {
	return &CommunityUpdater{
		updatedNodes: make(map[string]*corev1.Node),
		clearedNodes: make(map[string]*corev1.Node),
		updateNode:   updateNodeFunc,
		listNodes:    listNodesFunc,
		updateStatusNamespaceGenerator: func(ns string) UpdateStatusFunc {
			return eaclient.EdgeautoscalerV1alpha1().CommunityConfigurations(ns).Update
		},
	}
}

// CommunityName returns the name of a community
func CommunityName(c slpaclient.Community) string {
	return c.Name
}

// UpdateConfigurationStatus updates the status of cc resources with the new communities generated for a given namespace
func (c CommunityUpdater) UpdateConfigurationStatus(cc *eav1alpha1.CommunityConfiguration, communities []string) error {
	cc.Status.Communities = communities
	_, err := c.updateStatusNamespaceGenerator(cc.Namespace)(context.TODO(), cc, v1.UpdateOptions{})
	return err
}

// UpdateCommunityNodes applies the new labels for a given namespace to Kubernetes Nodes
func (c *CommunityUpdater) UpdateCommunityNodes(namespace string, communities []slpaclient.Community) error {
	clusterNodes, err := c.listNodes(labels.Everything())

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

			if comm, ok := labels[ealabels.CommunityLabel.WithNamespace(namespace).String()]; !ok || comm != community.Name {
				labels[ealabels.CommunityLabel.WithNamespace(namespace).String()] = community.Name
			}

			_, err := c.updateNode(context.TODO(), node, v1.UpdateOptions{})

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

		if _, ok := labels[ealabels.CommunityLabel.WithNamespace(namespace).String()]; !ok {
			continue
		}

		delete(labels, ealabels.CommunityLabel.WithNamespace(namespace).String())

		_, err := c.updateNode(context.TODO(), node, v1.UpdateOptions{})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error while deleting label from node Node %s: %s", node.Name, err))
			return err
		}

		c.clearedNodes[node.Name] = node
	}

	return nil
}

// ClearNodesLabels removes the labels of a given namespace from nodes
func (c *CommunityUpdater) ClearNodesLabels(namespace string) error {
	clusterNodes, err := c.listNodes(labels.Everything())

	if err != nil {
		return err
	}

	for _, node := range clusterNodes {
		if _, ok := node.Labels[ealabels.CommunityLabel.WithNamespace(namespace).String()]; ok {
			delete(node.Labels, ealabels.CommunityLabel.WithNamespace(namespace).String())
		}

		_, err = c.updateNode(context.TODO(), node, v1.UpdateOptions{})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error while deleting label from node Node %s: %s", node.Name, err))
			return err
		}
	}

	return nil
}
