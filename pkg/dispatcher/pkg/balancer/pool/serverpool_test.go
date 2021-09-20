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
					actual, found := p.GetBackend(desired.URL)
					require.True(t, found)
					require.Equal(t, desired, actual)
				}
			},
		},
		{
			description: "test RemoveBackend",
			input:       backends,

			verifyFunc: func(t *testing.T, p *ServerPool) {
				expectedLength := len(backends)
				for _, b := range backends {
					p.RemoveBackend(b)
					actual, found := p.GetBackend(b.URL)
					require.False(t, found)
					require.Equal(t, Backend{}, actual)

					require.Equal(t, expectedLength-1, len(p.backends))
					expectedLength--
				}
			},
		},
		{
			description: "test diff",
			input:       backends,

			verifyFunc: func(t *testing.T, p *ServerPool) {
				diff := p.BackendDiff([]*url.URL{backends[0].URL, backends[1].URL})
				require.Equal(t, []*url.URL{backends[2].URL}, diff)
			},
		},
		{
			description: "test diff",
			input:       []Backend{backends[0]},

			verifyFunc: func(t *testing.T, p *ServerPool) {
				diff := p.BackendDiff([]*url.URL{backends[0].URL, backends[1].URL})
				require.Equal(t, []*url.URL{}, diff)
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
