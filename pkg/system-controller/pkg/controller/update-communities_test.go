package controller

import (
	"context"
	"testing"

	eafake "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned/fake"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	"github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var resultObjectMeta = v1.ObjectMeta{
	Name: "node-1",
	Labels: map[string]string{
		ealabels.CommunityLabel.WithNamespace("").String(): "community-1",
		ealabels.WorkerLabel: ealabels.WorkerLabelValue,
	},
}

var notInCommunityMeta = v1.ObjectMeta{
	Name: "node-4",
	Labels: map[string]string{
		ealabels.CommunityLabel.WithNamespace("").String(): "community-2",
		ealabels.WorkerLabel: ealabels.WorkerLabelValue,
	},
}

func updateNode(ctx context.Context, node *corev1.Node, opts v1.UpdateOptions) (*corev1.Node, error) {
	return &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name:   resultObjectMeta.Name,
			Labels: resultObjectMeta.Labels,
		},
	}, nil
}

func listNodeWithNoLabel(selector labels.Selector) (ret []*corev1.Node, err error) {
	return []*corev1.Node{
		{
			ObjectMeta: v1.ObjectMeta{
				Name:   "node-1",
				Labels: map[string]string{},
			},
		},
	}, nil
}

func listNodeWithDifferentLabel(selector labels.Selector) (ret []*corev1.Node, err error) {
	return []*corev1.Node{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					ealabels.CommunityLabel.WithNamespace("").String(): "community-2",
					ealabels.WorkerLabel: ealabels.WorkerLabelValue,
				},
			},
		},
	}, nil
}

func listNodeNotInCommunity(selector labels.Selector) (ret []*corev1.Node, err error) {
	return []*corev1.Node{
		{
			ObjectMeta: notInCommunityMeta,
		},
	}, nil
}

func TestUpdateCommunityNodes(t *testing.T) {

	testcases := []struct {
		description         string
		input               []slpaclient.Community
		updateFunc          UpdateNodeFunc
		listFunc            ListNodeFunc
		desiredUpdatedNodes map[string]*corev1.Node
		desiredClearedNodes map[string]*corev1.Node
	}{
		{
			description: "Add to a node with no labels",
			input: []slpaclient.Community{
				{
					Name: "community-1",
					Members: []slpaclient.Host{
						{
							Name:   "node-1",
							Labels: map[string]interface{}{},
						},
					},
				},
			},
			updateFunc: updateNode,
			listFunc:   listNodeWithNoLabel,
			desiredUpdatedNodes: map[string]*corev1.Node{
				"node-1": {
					ObjectMeta: resultObjectMeta,
				},
			},
			desiredClearedNodes: map[string]*corev1.Node{},
		},
		{
			description: "Add to a node with different labels",
			input: []slpaclient.Community{
				{
					Name: "community-1",
					Members: []slpaclient.Host{
						{
							Name:   "node-1",
							Labels: map[string]interface{}{},
						},
					},
				},
			},
			updateFunc: updateNode,
			listFunc:   listNodeWithDifferentLabel,
			desiredUpdatedNodes: map[string]*corev1.Node{
				"node-1": {
					ObjectMeta: resultObjectMeta,
				},
			},
			desiredClearedNodes: map[string]*corev1.Node{},
		},
		{
			description: "Add to a node with different labels",
			input: []slpaclient.Community{
				{
					Name: "community-1",
					Members: []slpaclient.Host{
						{
							Name:   "node-1",
							Labels: map[string]interface{}{},
						},
					},
				},
			},
			updateFunc: updateNode,
			listFunc:   listNodeWithDifferentLabel,
			desiredUpdatedNodes: map[string]*corev1.Node{
				"node-1": {
					ObjectMeta: resultObjectMeta,
				},
			},
			desiredClearedNodes: map[string]*corev1.Node{},
		},
		{
			description: "clear node labels if not in community",
			input: []slpaclient.Community{
				{
					Name: "community-1",
					Members: []slpaclient.Host{
						{
							Name:   "node-1",
							Labels: map[string]interface{}{},
						},
					},
				},
			},
			updateFunc:          updateNode,
			listFunc:            listNodeNotInCommunity,
			desiredUpdatedNodes: map[string]*corev1.Node{},
			desiredClearedNodes: map[string]*corev1.Node{
				"node-4": {
					ObjectMeta: notInCommunityMeta,
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			c := NewCommunityUpdater(tt.updateFunc, tt.listFunc, eafake.NewSimpleClientset())

			err := c.UpdateCommunityNodes("", tt.input)

			require.Nil(t, err)
			require.Equal(t, tt.desiredUpdatedNodes, c.updatedNodes)
			require.Equal(t, tt.desiredClearedNodes, c.clearedNodes)
		})
	}
}
