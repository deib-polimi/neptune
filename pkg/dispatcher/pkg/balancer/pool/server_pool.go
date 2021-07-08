package pool

import (
	"log"
	"net"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/backend"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring"
)

// ServerPool is a collection of backends
type ServerPool struct {
	backends          []*backend.Backend
	current           uint64
	monitoringChannel chan monitoring.Metric
}

// NewServerPool returns a new ServerPool
func NewServerPool() *ServerPool {
	return &ServerPool{
		backends: []*backend.Backend{},
		current:  uint64(0),
	}
}

// AddBackend add a backend to the pool
func (s *ServerPool) AddBackend(b *backend.Backend) {
	s.backends = append(s.backends, b)
}

// RemoveBackend removes a backend to the pool
func (s *ServerPool) RemoveBackend(b *backend.Backend) {
	for i, backend := range s.backends {
		if backend.URL == b.URL {
			s.backends = append(s.backends[:i], s.backends[i+1:]...)
		}
	}
}

func (s *ServerPool) nextIndex() int {
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

// GetNextPeer returns the backend that should serve the new request
func (s *ServerPool) GetNextPeer() *backend.Backend {
	next := s.nextIndex()
	l := len(s.backends) + next

	for i := next; i < l; i++ {
		idx := i % len(s.backends)

		if s.backends[idx].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}

	}

	return nil
}

// HealthCheck pings the backends and update the status
func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
		status := "up"
		alive := s.isBackendAlive(b.URL)
		b.SetAlive(alive)
		if !alive {
			//TODO: notify the controller if the host is down
			status = "down"
		}
		log.Printf("%s [%s]\n", b.URL, status)
	}
}

// isAlive checks whether a backend is Alive by establishing a TCP connection
func (s *ServerPool) isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Println("backend unreachable, error: ", err)
		return false
	}
	_ = conn.Close() // close it, we dont need to maintain this connection
	return true
}

// StoreBackendStatus set the current status of a backend
func (s *ServerPool) StoreBackendStatus(backendURL *url.URL, status bool) {
	for _, backend := range s.backends {
		if backend.URL.String() == backendURL.String() {
			backend.SetAlive(status)
			break
		}
	}
}
