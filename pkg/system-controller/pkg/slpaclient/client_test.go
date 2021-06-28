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
			description: "",
			input:       []byte(""),
			desired: []Community{
				{
					Name: "",
					Members: []Host{
						{
							Name:   "",
							Labels: make(map[string]interface{}),
						},
					},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			client := NewClient()

			actual, err := client.parseRawCommunities(tt.input)

			require.Nil(t, err)
			require.Equal(t, tt.desired, actual)
		})
	}
}
