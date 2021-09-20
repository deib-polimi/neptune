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
	lock     *sync.RWMutex
}

// NewServerPool returns a new ServerPool
func NewServerPool() *ServerPool {
	return &ServerPool{
		backends: []ru.Choice{},
		lock:     &sync.RWMutex{},
	}
}

// SetBackend set a backend  in the map and its workload weight
func (s *ServerPool) SetBackend(b Backend, weigth int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	klog.Infof("set backend %v", b)

	for idx, backend := range s.backends {
		if backend.Item.(Backend).URL.String() == b.URL.String() {
			s.backends[idx] = ru.Choice{Item: b, Weight: weigth}
			return
		}
	}
	s.backends = append(s.backends, ru.Choice{Weight: weigth, Item: b})
}

// RemoveBackend removes a backend from the pool
func (s *ServerPool) RemoveBackend(b Backend) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for i, choice := range s.backends {
		if choice.Item == b {
			s.backends[i] = s.backends[len(s.backends)-1]
			s.backends = s.backends[:len(s.backends)-1]
			break
		}
	}

	if len(s.backends) == 0 {
		klog.Errorf("Empty backends. Last backend removed is: %v", b)
	}
}

// GetBackend returns a backend given its URL. It returns the backend and bool
// if the server exists
func (s *ServerPool) GetBackend(url *url.URL) (backend Backend, found bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	found = false
	backend = Backend{}
	for _, choice := range s.backends {
		if choice.Item.(Backend).URL.String() == url.String() {
			backend = choice.Item.(Backend)
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
	diff = []*url.URL{}
	blueprint := make(map[string]*url.URL, len(s.backends))

	for _, choice := range s.backends {
		klog.Infof("%v", choice.Item.(Backend).URL.Host)
		blueprint[choice.Item.(Backend).URL.String()] = choice.Item.(Backend).URL
	}

	for _, desired := range backends {
		if _, exists := blueprint[desired.String()]; exists {
			delete(blueprint, desired.String())
		}
	}

	for _, oldBackend := range blueprint {
		diff = append(diff, oldBackend)
	}
	return

}
