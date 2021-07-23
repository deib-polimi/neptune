package pool

import (
	"net/http/httputil"
	"net/url"

	"k8s.io/apimachinery/pkg/api/resource"
)

//TODO: maybe resource.Quantity will not be used by default

// Backend is the default backend wrapper
type Backend struct {
	URL *url.URL

	ReverseProxy *httputil.ReverseProxy
}

// ServerPool contains the backends list
type ServerPool struct {
	backends map[*Backend]*resource.Quantity
}

// NewServerPool returns a new ServerPool
func NewServerPool() *ServerPool {
	return &ServerPool{
		backends: make(map[*Backend]*resource.Quantity),
	}
}

// SetBackend set a backend  in the map and its workload weight
func (s *ServerPool) SetBackend(b *Backend, workload *resource.Quantity) {
	s.backends[b] = workload
}

// RemoveBackend removes a backend from the pool
func (s *ServerPool) RemoveBackend(b *Backend) {
	delete(s.backends, b)
}

// GetBackend returns a backend given its URL. It returns the backend and bool
// if the server exists
func (s *ServerPool) GetBackend(url *url.URL) (backend *Backend, found bool) {
	for b := range s.backends {
		if b.URL == url {
			return b, true
		}
	}
	return nil, false
}

// NextBackend returns the backend that should serve the new request
func (s *ServerPool) NextBackend() (b *Backend) {
	//TODO: add logic to choose the best backend
	for backend := range s.backends {
		b = backend
		break
	}
	return b
}

func (s *ServerPool) BackendDiff(backends []*url.URL) (diff []*url.URL) {
	blueprint := make(map[*url.URL]bool)

	for actual := range s.backends {
		blueprint[actual.URL] = true
	}

	for _, desired := range backends {
		if _, exists := blueprint[desired]; exists {
			delete(blueprint, desired)
		}
	}

	for oldBackend := range blueprint {
		diff = append(diff, oldBackend)
	}
	return

}
