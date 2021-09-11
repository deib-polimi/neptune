package balancer

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/pool"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/queue"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
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

// NodeInfo wraps the info needed by the persistor to save metrics
type NodeInfo struct {
	// Node is the name of the node on which the balancer is running
	Node string
	// Function is the name of the function to be balanced
	Function string
	// Namespace is the namespace of the function
	Namespace string
	// Community is the community on which the function is running
	Community string
}

// LoadBalancer is the wrapper of the server pool and performs the routing to any of its backends
type LoadBalancer struct {
	serverPool *pool.ServerPool
	metricChan chan<- metrics.RawResponseTime
	NodeInfo
	// metricChan chan<- monitoringmetrics.RawMetricData
}

// NewLoadBalancer returns a new LoadBalancer
func NewLoadBalancer(node NodeInfo, metricChan chan<- metrics.RawResponseTime) *LoadBalancer {
	// func NewLoadBalancer(monitoringChan chan<- monitoringmetrics.RawMetricData) *LoadBalancer {
	lb := &LoadBalancer{
		NodeInfo:   node,
		serverPool: pool.NewServerPool(),
		metricChan: metricChan,
	}

	return lb
}

// Balance forwards to any of the active backends the incoming request
func (lb *LoadBalancer) Balance(w http.ResponseWriter, r *http.Request) {
	peer, err := lb.serverPool.NextBackend()

	if err != nil {
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	transformer := UpstreamRequestBuilder{
		Request: r,
		Backend: peer,
	}

	upstreamReq := transformer.Build()

	if upstreamReq.Body != nil {
		defer upstreamReq.Body.Close()
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*90)
	defer cancel()

	start := time.Now()
	res, resErr := peer.Client.Do(upstreamReq.WithContext(ctx))

	delta := time.Since(start)

	requestData := metrics.RawResponseTime{
		Timestamp:   start,
		Source:      lb.Node,
		Function:    lb.Function,
		Destination: peer.Node,
		Namespace:   lb.Namespace,
		Community:   lb.Community,
		Gpu:         peer.HasGpu,
		Latency:     int(delta.Milliseconds()),
	}

	if resErr != nil {
		requestData.StatusCode = http.StatusBadGateway
		requestData.Description = resErr.Error()
	} else {
		requestData.StatusCode = res.StatusCode
		bodyBytes, err := ioutil.ReadAll(res.Body)

		if err != nil {
			requestData.Description = err.Error()
		} else {
			requestData.Description = string(bodyBytes)
		}
	}

	lb.metricChan <- requestData

	if resErr != nil {
		http.Error(w, "Bad gateway", http.StatusBadGateway)
		return
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	copyHeaders(w.Header(), &res.Header)

	// Write status code
	w.WriteHeader(res.StatusCode)

	if res.Body != nil {
		// Copy the body over
		io.CopyBuffer(w, res.Body, nil)
	}

	klog.Info("request served")

	return

}

// AddServer adds a new backend to the server pool
func (lb *LoadBalancer) AddServer(serverURL *url.URL, node string, hasGpu bool, workload *resource.Quantity, recovery recoveryFunc) {

	// proxy := httputil.NewSingleHostReverseProxy(serverURL)
	// // TODO: this does not work
	// proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
	// 	utilruntime.HandleError(fmt.Errorf("error while serving request to backend %v: %v", request.URL.Host, e))
	// 	// enqueue the request again if it cannot be served by a backend
	// 	http.Error(writer, "Service not available", http.StatusServiceUnavailable)
	// }
	// proxy.Transport = &http.Transport{
	// 	DialContext: (&net.Dialer{
	// 		KeepAlive: 5 * time.Minute,
	// 		Timeout:   90 * time.Second,
	// 	}).DialContext,
	// }

	b := pool.Backend{
		URL:    serverURL,
		Node:   node,
		HasGpu: hasGpu,
		Client: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					KeepAlive: 5 * time.Minute,
					Timeout:   90 * time.Second,
				}).DialContext,
				IdleConnTimeout:       90 * time.Second,
				ExpectContinueTimeout: 5 * time.Second,
			},
			Timeout: time.Second * 10,
		},
	}

	lb.serverPool.SetBackend(b, int(workload.MilliValue()))
}

// DeleteServer removes a backend from the pool
func (lb *LoadBalancer) DeleteServer(serverURL *url.URL) error {
	b, _, exists := lb.serverPool.GetBackend(serverURL)

	if !exists {
		return fmt.Errorf(ServerNotFoundError, serverURL.Host)
	}

	lb.serverPool.RemoveBackend(b)
	return nil
}

// ServerExists checks if a backend exists in the pool
func (lb *LoadBalancer) ServerExists(serverURL *url.URL) (exists bool) {
	_, _, exists = lb.serverPool.GetBackend(serverURL)
	return exists
}

// UpdateWorkload set the new server weight
func (lb *LoadBalancer) UpdateWorkload(serverURL *url.URL, workload *resource.Quantity) error {
	b, _, exists := lb.serverPool.GetBackend(serverURL)

	if !exists {
		return fmt.Errorf(ServerNotFoundError, serverURL.Host)
	}

	lb.serverPool.SetBackend(b, int(workload.MilliValue()))
	return nil
}

func (lb *LoadBalancer) ServerPoolDiff(servers []*url.URL) []*url.URL {
	return lb.serverPool.BackendDiff(servers)
}

// Shutdown is called when the controller has finished its work
func (lb *LoadBalancer) Shutdown() {
	utilruntime.HandleCrash()
}
