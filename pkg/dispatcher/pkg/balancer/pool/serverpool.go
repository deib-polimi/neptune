package pool

import (
	"net/http"
	"net/url"

	"github.com/modern-go/concurrent"
	"k8s.io/apimachinery/pkg/api/resource"
)

//TODO: maybe resource.Quantity will not be used by default

// Backend is the default backend wrapper
type Backend struct {
	// URL is the pod URL
	URL *url.URL
	// Node is the node name on which the pod is running
	Node string
	// HasGpu is used to determine if the pod has a GPU attached
	HasGpu bool
	// Client is the http client to use to connect to the pod
	Client *http.Client
}

// ServerPool contains the backends list
type ServerPool struct {
	backends concurrent.Map
}

// NewServerPool returns a new ServerPool
func NewServerPool() *ServerPool {
	return &ServerPool{
		backends: concurrent.Map{},
	}
}

// SetBackend set a backend  in the map and its workload weight
func (s *ServerPool) SetBackend(b Backend, workload *resource.Quantity) {
	s.backends.Store(b, workload)
}

// RemoveBackend removes a backend from the pool
func (s *ServerPool) RemoveBackend(b Backend) {
	s.backends.Delete(b)
}

// GetBackend returns a backend given its URL. It returns the backend and bool
// if the server exists
func (s *ServerPool) GetBackend(url *url.URL) (backend Backend, found bool) {
	found = false
	s.backends.Range(func(key, value interface{}) bool {
		if key.(Backend).URL.String() == url.String() {
			backend = key.(Backend)
			found = true
			return false
		}
		return true
	})

	if !found {
		return Backend{}, false
	}

	return
}

// NextBackend returns the backend that should serve the new request
func (s *ServerPool) NextBackend() (backend Backend, err error) {
	s.backends.Range(func(key, value interface{}) bool {
		backend = key.(Backend)
		return false
	})

	return backend, nil
}

func (s *ServerPool) BackendDiff(backends []*url.URL) (diff []*url.URL) {
	blueprint := make(map[*url.URL]bool)

	s.backends.Range(func(key, value interface{}) bool {
		blueprint[key.(Backend).URL] = true
		return true
	})

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
