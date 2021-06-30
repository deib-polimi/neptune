package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var readyNode = &corev1.Node{
	ObjectMeta: v1.ObjectMeta{
		Name: "node-1",
	},
	Status: corev1.NodeStatus{
		Conditions: []corev1.NodeCondition{
			{
				Type:              corev1.NodeReady,
				Status:            corev1.ConditionTrue,
				LastHeartbeatTime: v1.Now(),
			},
		},
	},
}

var notReadyNode = &corev1.Node{
	ObjectMeta: v1.ObjectMeta{
		Name: "node-2",
	},
	Status: corev1.NodeStatus{
		Conditions: []corev1.NodeCondition{
			{
				Type:              corev1.NodeReady,
				Status:            corev1.ConditionFalse,
				LastHeartbeatTime: v1.Now(),
			},
		},
	},
}

var unknownNode = &corev1.Node{
	ObjectMeta: v1.ObjectMeta{
		Name: "node-3",
	},
	Status: corev1.NodeStatus{
		Conditions: []corev1.NodeCondition{
			{
				Type:              corev1.NodeReady,
				Status:            corev1.ConditionUnknown,
				LastHeartbeatTime: v1.Now(),
			},
		},
	},
}

func TestFilterReadyNodes(t *testing.T) {
	testcases := []struct {
		description string
		input       []*corev1.Node
		desired     []*corev1.Node
		verifyFunc  func([]*corev1.Node, []*corev1.Node, error)
	}{
		{
			description: "return all ready nodes",
			input:       []*corev1.Node{readyNode},
			desired:     []*corev1.Node{readyNode},
			verifyFunc: func(desiredNodes []*corev1.Node, actualNodes []*corev1.Node, err error) {
				require.Nil(t, err, "no errors should occur")
				require.ElementsMatch(t, desiredNodes, actualNodes)
			},
		},
		{
			description: "do not return nodes with ready condition to false or unknown",
			input:       []*corev1.Node{readyNode, unknownNode, notReadyNode},
			desired:     []*corev1.Node{readyNode},
			verifyFunc: func(desiredNodes []*corev1.Node, actualNodes []*corev1.Node, err error) {
				require.Nil(t, err, "no errors should occur")
				require.ElementsMatch(t, desiredNodes, actualNodes)
				require.NotContains(t, actualNodes, unknownNode)
				require.NotContains(t, actualNodes, notReadyNode)
			},
		},
		{
			description: "error is returned is there are no ready nodes",
			input:       []*corev1.Node{unknownNode, notReadyNode},
			desired:     []*corev1.Node{},
			verifyFunc: func(desiredNodes []*corev1.Node, actualNodes []*corev1.Node, err error) {
				require.Equal(t, err.Error(), EmptyNodeList)
				require.Empty(t, actualNodes)
				require.NotContains(t, actualNodes, unknownNode)
				require.NotContains(t, actualNodes, notReadyNode)
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual, err := filterReadyNodes(tt.input)
			tt.verifyFunc(tt.desired, actual, err)
		})
	}
}

func TestGetNodeDelays(t *testing.T) {
	testcases := []struct {
		description string
		input       []*corev1.Node
		desired     [][]int32
	}{
		{
			description: "return the node matrix",
			input:       []*corev1.Node{readyNode, unknownNode, notReadyNode},
			desired: [][]int32{
				{0, 2, 2},
				{2, 0, 2},
				{2, 2, 0},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			c := SystemController{}
			actual, err := c.getNodeDelays(tt.input)
			require.Nil(t, err)
			require.Equal(t, tt.desired, actual)

			for node := range actual {
				require.Zero(t, actual[node][node])
				require.Equal(t, len(actual), len(actual[node]))
			}
		})
	}
}
