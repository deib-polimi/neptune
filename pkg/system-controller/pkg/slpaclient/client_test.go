package slpaclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRawCommunities(t *testing.T) {
	testcases := []struct {
		description string
		input       []byte
		desired     []Community
	}{
		{
			description: "basic test",
			input: []byte(`{
				"communities": [
						{
								"name": "community-5",
								"members": [
										{
												"name": "node-5",
												"labels": {
														"edgeautoscaler.polimi.it/role": "LEADER"
												}
										}
								]
						}
				]
		}`),
			desired: []Community{
				{
					Name: "community-5",
					Members: []Host{
						{
							Name: "node-5",
							Labels: map[string]interface{}{
								"edgeautoscaler.polimi.it/role": "LEADER",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			client := NewClient("")

			actual, err := client.parseRawCommunities(tt.input)

			require.Nil(t, err)
			require.Equal(t, tt.desired, actual)
		})
	}
}
