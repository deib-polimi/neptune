package balancer

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/pool"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/queue"
	monitoringmetrics "github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	"k8s.io/apimachinery/pkg/api/resource"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

type requestCount int

// recoveryFunc handles the request forwarding failure
type recoveryFunc func(req *queue.HTTPRequest)

const (
	// Attempts represents how many times the balancer has tried to forward a request to a backend and did not succeded because it was unreachable
	Attempts requestCount = iota
	// Retry represents how many times the balancer has tried to add a backend to the server pool and did not succeded because it was unreachable
	Retry

	// ServerNotFoundError is the standard error when trying to manipulate a non existing server
	ServerNotFoundError string = "server %s does not exists"
)

// LoadBalancer is the wrapper of the server pool and performs the routing to any of its backends
type LoadBalancer struct {
	serverPool *pool.ServerPool
	metricChan chan<- monitoringmetrics.RawMetricData
}

// NewLoadBalancer returns a new LoadBalancer
func NewLoadBalancer(monitoringChan chan<- monitoringmetrics.RawMetricData) *LoadBalancer {
	lb := &LoadBalancer{
		serverPool: pool.NewServerPool(),
		metricChan: monitoringChan,
	}

	return lb
}

// Balance forwards to any of the active backends the incoming request
func (lb *LoadBalancer) Balance(w http.ResponseWriter, r *http.Request) {
	peer, err := lb.serverPool.NextBackend()

	if err != nil {
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
	}

	klog.Infof("Serving request: %v forwarding to pod: %v", r.URL.Scheme+"//"+r.Host+r.URL.Path, peer.URL.Host)
	requestTime := time.Now()
	peer.ReverseProxy.ServeHTTP(w, r)
	responseTime := time.Now()
	delta := responseTime.Sub(requestTime)
	lb.metricChan <- monitoringmetrics.RawMetricData{
		Backend:     peer.URL,
		Value:       float64(delta.Milliseconds()),
		FunctionURL: r.URL.Host,
	}
	return

}

// AddServer adds a new backend to the server pool
func (lb *LoadBalancer) AddServer(serverURL *url.URL, workload *resource.Quantity, recovery recoveryFunc) {
	proxy := httputil.NewSingleHostReverseProxy(serverURL)
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
		utilruntime.HandleError(fmt.Errorf("error while serving request to backend %v: %v", request.URL.Host, e))
		// enqueue the request again if it cannot be served by a backend
		recovery(&queue.HTTPRequest{
			ResponseWriter: writer,
			Request:        request,
		})
	}

	b := pool.Backend{
		URL:          serverURL,
		ReverseProxy: proxy,
	}

	lb.serverPool.SetBackend(b, workload)
}

// DeleteServer removes a backend from the pool
func (lb *LoadBalancer) DeleteServer(serverURL *url.URL) error {
	b, exists := lb.serverPool.GetBackend(serverURL)

	if !exists {
		return fmt.Errorf(ServerNotFoundError, serverURL.Host)
	}

	lb.serverPool.RemoveBackend(b)
	return nil
}

// ServerExists checks if a backend exists in the pool
func (lb *LoadBalancer) ServerExists(serverURL *url.URL) (exists bool) {
	_, exists = lb.serverPool.GetBackend(serverURL)
	return exists
}

// UpdateWorkload set the new server weight
func (lb *LoadBalancer) UpdateWorkload(serverURL *url.URL, workload *resource.Quantity) error {
	b, exists := lb.serverPool.GetBackend(serverURL)

	if !exists {
		return fmt.Errorf(ServerNotFoundError, serverURL.Host)
	}

	lb.serverPool.SetBackend(b, workload)
	return nil
}

func (lb *LoadBalancer) ServerPoolDiff(servers []*url.URL) []*url.URL {
	return lb.serverPool.BackendDiff(servers)
}

// Shutdown is called when the controller has finished its work
func (lb *LoadBalancer) Shutdown() {
	utilruntime.HandleCrash()
}
