package controller

import (
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var masterNode = &corev1.Node{
	ObjectMeta: metav1.ObjectMeta{
		Name: "master",
		Labels: map[string]string{
			ealabels.MasterNodeLabel: "",
		},
	},
	Status: corev1.NodeStatus{
		Conditions: []corev1.NodeCondition{
			{
				Type:              corev1.NodeReady,
				Status:            corev1.ConditionTrue,
				LastHeartbeatTime: metav1.Now(),
			},
		},
	},
}

var readyNode = &corev1.Node{
	ObjectMeta: metav1.ObjectMeta{
		Name:   "node-1",
		Labels: make(map[string]string),
	},
	Status: corev1.NodeStatus{
		Conditions: []corev1.NodeCondition{
			{
				Type:              corev1.NodeReady,
				Status:            corev1.ConditionTrue,
				LastHeartbeatTime: metav1.Now(),
			},
		},
	},
}

var notReadyNode = &corev1.Node{
	ObjectMeta: metav1.ObjectMeta{
		Name:   "node-2",
		Labels: make(map[string]string),
	},
	Status: corev1.NodeStatus{
		Conditions: []corev1.NodeCondition{
			{
				Type:              corev1.NodeReady,
				Status:            corev1.ConditionFalse,
				LastHeartbeatTime: metav1.Now(),
			},
		},
	},
}

var unknownNode = &corev1.Node{
	ObjectMeta: metav1.ObjectMeta{
		Name:   "node-3",
		Labels: make(map[string]string),
	},
	Status: corev1.NodeStatus{
		Conditions: []corev1.NodeCondition{
			{
				Type:              corev1.NodeReady,
				Status:            corev1.ConditionUnknown,
				LastHeartbeatTime: metav1.Now(),
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
			description: "return all ready nodes except master",
			input:       []*corev1.Node{readyNode, masterNode},
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
				require.Equal(t, err.Error(), EmptyNodeListError)
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

func TestComputeDeploymentReplicas(t *testing.T) {

	testcases := []struct {
		description string
		communities []string
		replicas    []int
		expected    int32
		error       bool
	}{
		{
			description: "single community",
			communities: []string{"community-1"},
			replicas:    []int{1},
			expected:    1,
			error:       false,
		},
		{
			description: "multi-community",
			communities: []string{"community-1", "community-2", "community-3"},
			replicas:    []int{1, 2, 3},
			expected:    6,
			error:       false,
		},
		{
			description: "multi-community big test",
			communities: []string{"community-1", "community-2", "community-3", "community-4", "community-5", "community-6"},
			replicas:    []int{1, 2, 3, 4, 5, 6},
			expected:    21,
			error:       false,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			deployment := newDeployment()
			for i, community := range tt.communities {
				l := ealabels.CommunityInstancesLabel.WithNamespace("new-namespace").WithName(community).String()
				deployment.Labels[l] = strconv.Itoa(tt.replicas[i])
			}
			replicas, err := ComputeDeploymentReplicas(deployment, "new-namespace", tt.communities)
			if err != nil {
				require.True(t, tt.error)
			} else {
				require.False(t, tt.error)
				require.Equal(t, *replicas, tt.expected)
			}
		})
	}
}

func newDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: make(map[string]string),
		},
	}
}
