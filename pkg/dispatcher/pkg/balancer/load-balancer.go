package balancer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
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

	upstreamReq := buildUpstreamRequest(r, fmt.Sprintf("%v://%v", peer.URL.Scheme, peer.URL.Host), "")

	if upstreamReq.Body != nil {
		defer upstreamReq.Body.Close()
	}

	log.Printf("forwardRequest: %s %s\n", upstreamReq.Host, upstreamReq.URL.String())

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*30)
	defer cancel()

	res, resErr := http.DefaultClient.Do(upstreamReq.WithContext(ctx))

	if resErr != nil {
		badStatus := http.StatusBadGateway
		w.WriteHeader(badStatus)
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

	// // TODO: move in a URLTransformer struct to change the host and remove /functino/<namespace>/<function> from path
	// reqClone := r.Clone(context.Background())
	// reqClone.Host = peer.URL.Host
	// reqClone.URL.Path = ""
	// reqClone.URL.Scheme = "http"
	// klog.Infof("Serving request: %v forwarding to pod: %v", reqClone.URL.Scheme+"//"+reqClone.Host+reqClone.URL.Path, peer.URL.Host)

	// requestTime := time.Now()
	// res, err := http.DefaultClient.Do(reqClone)
	// if err != nil {
	// 	klog.Errorf("Error forwarding request: %v", err)
	// 	return
	// }

	// copyHeaders(w.Header(), &res.Header)

	// // Write status code
	// w.WriteHeader(res.StatusCode)

	// if res.Body != nil {
	// 	// Copy the body over
	// 	io.CopyBuffer(w, res.Body, nil)
	// }

	// // peer.ReverseProxy.ServeHTTP(w, reqClone)
	// responseTime := time.Now()
	// delta := responseTime.Sub(requestTime)
	// lb.metricChan <- monitoringmetrics.RawMetricData{
	// 	Backend:     peer.URL,
	// 	Value:       float64(delta.Milliseconds()),
	// 	FunctionURL: r.URL.Host,
	// }
	klog.Info("request served")

	return

}
func copyHeaders(destination http.Header, source *http.Header) {
	for k, v := range *source {
		vClone := make([]string, len(v))
		copy(vClone, v)
		(destination)[k] = vClone
	}
}
func deleteHeaders(target *http.Header, exclude *[]string) {
	for _, h := range *exclude {
		target.Del(h)
	}
}

func buildUpstreamRequest(r *http.Request, baseURL string, requestURL string) *http.Request {
	url := baseURL + requestURL

	if len(r.URL.RawQuery) > 0 {
		url = fmt.Sprintf("%s?%s", url, r.URL.RawQuery)
	}

	// upstreamReq, err := http.NewRequest(http.MethodPost, url, nil)
	upstreamReq, err := http.NewRequest(r.Method, url, nil)

	if err != nil {
		klog.Errorf("Error creating upstream request: %v", err)
		return nil
	}

	copyHeaders(upstreamReq.Header, &r.Header)

	if len(r.Host) > 0 && upstreamReq.Header.Get("X-Forwarded-Host") == "" {
		upstreamReq.Header["X-Forwarded-Host"] = []string{r.Host}
	}

	if upstreamReq.Header.Get("X-Forwarded-For") == "" {
		upstreamReq.Header["X-Forwarded-For"] = []string{r.RemoteAddr}
	}

	if r.Body != nil {
		upstreamReq.Body = r.Body
	}

	return upstreamReq
}

// AddServer adds a new backend to the server pool
func (lb *LoadBalancer) AddServer(serverURL *url.URL, workload *resource.Quantity, recovery recoveryFunc) {
	klog.Infof("Adding server: %v", serverURL.String())
	proxy := httputil.NewSingleHostReverseProxy(serverURL)
	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 90 * time.Second,
		}).DialContext,
		// TODO: Some of those value should be tuned
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		MaxIdleConnsPerHost:   1000,
		MaxIdleConns:          1000,
	}

	// TODO: this does not work
	// proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
	// 	utilruntime.HandleError(fmt.Errorf("error while serving request to backend %v: %v", request.URL.Host, e))
	// 	// enqueue the request again if it cannot be served by a backend
	// 	recovery(&queue.HTTPRequest{
	// 		ResponseWriter: writer,
	// 		Request:        request,
	// 	})
	// }

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
