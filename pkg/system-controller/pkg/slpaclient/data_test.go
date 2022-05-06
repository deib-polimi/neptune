package slpaclient

import (
	"testing"

	eaapi "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var cc = &eaapi.CommunityConfiguration{
	TypeMeta: v1.TypeMeta{
		Kind:       "CommunityConfiguration",
		APIVersion: eaapi.SchemeGroupVersion.Identifier(),
	},
	Spec: communityConfigSpec,
}

var delays = [][]int64{
	{0, 2, 2},
	{2, 0, 2},
	{2, 2, 0},
}

var communityConfigSpec = eaapi.CommunityConfigurationSpec{
	SlpaService:          "foo.bar",
	CommunitySize:        2,
	MaximumDelay:         40,
	ProbabilityThreshold: 50,
	Iterations:           20,
}

var nodes = []*corev1.Node{
	{
		ObjectMeta: v1.ObjectMeta{
			Name: "node-1",
		},
	},
	{
		ObjectMeta: v1.ObjectMeta{
			Name: "node-2",
		},
	},
	{
		ObjectMeta: v1.ObjectMeta{
			Name: "node-3",
		},
	},
}

func TestNewRequestSLPA(t *testing.T) {
	testcases := []struct {
		description string
		inputCC     *eaapi.CommunityConfiguration
		inputNodes  []*corev1.Node
		inputDelays [][]int64
		desired     *RequestSLPA
	}{
		{
			description: "return the node matrix",
			inputCC: &eaapi.CommunityConfiguration{
				Spec: communityConfigSpec,
			},
			inputNodes:  nodes,
			inputDelays: delays,
			desired: &RequestSLPA{
				Parameters: ParametersSLPA{
					CommunitySize:        communityConfigSpec.CommunitySize,
					MaximumDelay:         communityConfigSpec.MaximumDelay,
					ProbabilityThreshold: communityConfigSpec.ProbabilityThreshold,
					Iterations:           communityConfigSpec.Iterations,
				},
				Hosts: []Host{
					{
						Name:   "node-1",
						Labels: map[string]interface{}{},
					},
					{
						Name:   "node-2",
						Labels: map[string]interface{}{},
					},
					{
						Name:   "node-3",
						Labels: map[string]interface{}{},
					},
				},
				DelayMatrix: DelayMatrix{
					[][]int64{
						{0, 2, 2},
						{2, 0, 2},
						{2, 2, 0},
					},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {

			actual := NewRequestSLPA(tt.inputCC, tt.inputNodes, tt.inputDelays)
			require.Equal(t, tt.desired, actual)
			require.ElementsMatch(t, tt.desired.Hosts, actual.Hosts)
			require.ElementsMatch(t, tt.desired.DelayMatrix.Delays, actual.DelayMatrix.Delays)
		})
	}
}
