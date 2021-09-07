package pool

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"k8s.io/klog/v2"

	ru "github.com/jmcvetta/randutil"
)

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

// ServerPool contains a set of backends grouped by the service URL they are serving
type ServerPool struct {
	backends []ru.Choice
	lock     sync.RWMutex
}

// NewServerPool returns a new ServerPool
func NewServerPool() *ServerPool {
	return &ServerPool{}
}

// SetBackend set a backend  in the map and its workload weight
func (s *ServerPool) SetBackend(b Backend, weigth int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	klog.Infof("adding backend %v", b)

	for _, backend := range s.backends {
		if backend.Item.(Backend).URL.String() == b.URL.String() {
			s.removeBackend(b)
			break
		}
	}
	s.backends = append(s.backends, ru.Choice{Weight: weigth, Item: b})
}

// RemoveBackend removes a backend from the pool
func (s *ServerPool) RemoveBackend(b Backend) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.removeBackend(b)
}

func (s *ServerPool) removeBackend(b Backend) {
	for i, choice := range s.backends {
		if choice.Item == b {
			s.backends[i] = s.backends[len(s.backends)-1]
			s.backends = s.backends[:len(s.backends)-1]
			break
		}
	}
}

// GetBackend returns a backend given its URL. It returns the backend and bool
// if the server exists
func (s *ServerPool) GetBackend(url *url.URL) (backend Backend, weight int, found bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	found = false
	backend = Backend{}
	weight = 0
	for _, choice := range s.backends {
		if choice.Item.(Backend).URL.String() == url.String() {
			backend = choice.Item.(Backend)
			weight = choice.Weight
			found = true
			return
		}
	}

	return
}

// NextBackend returns the backend that should serve the new request
func (s *ServerPool) NextBackend() (Backend, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	choice, err := ru.WeightedChoice(s.backends)
	if err != nil {
		return Backend{}, fmt.Errorf("failed to get a random backend, error: %s", err)
	}
	backend := choice.Item.(Backend)
	return backend, nil
}

func (s *ServerPool) BackendDiff(backends []*url.URL) (diff []*url.URL) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	blueprint := make(map[*url.URL]bool)

	for _, choice := range s.backends {
		blueprint[choice.Item.(Backend).URL] = true
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
