package slpaclient

import (
	"testing"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	"github.com/stretchr/testify/require"
)

func TestFakeCommunities(t *testing.T) {
	testcases := []struct {
		description string
		input       *RequestSLPA
		desired     []Community
	}{
		{
			description: "testing the fake client",
			input:       NewRequestSLPA(cc, nodes, delays),
			desired: []Community{
				{
					Name: "community-0",
					Members: []Host{
						{
							Name: "node-1",
							Labels: map[string]interface{}{
								ealabels.CommunityRoleLabel.String(): ealabels.Leader.String(),
								ealabels.CommunityLabel.String():     "community-0",
							},
						},
						{
							Name: "node-3",
							Labels: map[string]interface{}{
								ealabels.CommunityRoleLabel.String(): ealabels.Member.String(),
								ealabels.CommunityLabel.String():     "community-0",
							},
						},
					},
				},
				{
					Name: "community-1",
					Members: []Host{
						{
							Name: "node-2",
							Labels: map[string]interface{}{
								ealabels.CommunityRoleLabel.String(): ealabels.Leader.String(),
								ealabels.CommunityLabel.String():     "community-1",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			fc := NewFakeClient()

			actual, err := fc.Communities(tt.input)

			require.Nil(t, err)
			require.Equal(t, tt.desired, actual)
		})
	}
}
