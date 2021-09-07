package pool

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

var backends = []Backend{
	{
		URL: &url.URL{
			Host: "localhost:8080",
		},
	},
	{
		URL: &url.URL{
			Host: "localhost:8081",
		},
	},
	{
		URL: &url.URL{
			Host: "localhost:8082",
		},
	},
}

func TestPool(t *testing.T) {
	testcases := []struct {
		description string
		input       []Backend
		verifyFunc  func(t *testing.T, p *ServerPool)
	}{
		{
			description: "test GetBackend",
			input:       backends,

			verifyFunc: func(t *testing.T, p *ServerPool) {
				for _, desired := range backends {
					actual, _, found := p.GetBackend(desired.URL)
					require.True(t, found)
					require.Equal(t, desired, actual)
				}
			},
		},
		{
			description: "test RemoveBackend",
			input:       backends,

			verifyFunc: func(t *testing.T, p *ServerPool) {
				for _, b := range backends {
					p.RemoveBackend(b)
					actual, _, found := p.GetBackend(b.URL)
					require.False(t, found)
					require.Equal(t, Backend{}, actual)
				}
			},
		},
		{
			description: "update backend weight",
			input:       backends,

			verifyFunc: func(t *testing.T, p *ServerPool) {
				p.SetBackend(backends[0], 1)
				b, weight, found := p.GetBackend(backends[0].URL)
				require.True(t, found)
				require.Equal(t, backends[0], b)
				require.Equal(t, 1, weight)
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			pool := NewServerPool()

			for _, backend := range tt.input {
				pool.SetBackend(backend, 2)
			}

			tt.verifyFunc(t, pool)
		})
	}
}
