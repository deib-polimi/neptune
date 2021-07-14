package controller

import (
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"testing"
)

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
				l := CommunityInstancesLabel.WithNamespace("new-namespace").WithName(community).String()
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
