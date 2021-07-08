package balancer

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/backend"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/pool"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type requestCount int

const (
	// Attempts represents how many times the balancer has tried to forward a request to a backend and did not succeded because it was unreachable
	Attempts requestCount = iota
	// Retry represents how many times the balancer has tried to add a backend to the server pool and did not succeded because it was unreachable
	Retry
)

// LoadBalancer is the wrapper of the server pool and performs the routing to any of its backends
type LoadBalancer struct {
	serverPool pool.ServerPool
	metricChan chan<- monitoring.Metric
}

// NewLoadBalancer returns a new LoadBalancer
func NewLoadBalancer(monitoringChan chan<- monitoring.Metric) *LoadBalancer {
	return &LoadBalancer{
		serverPool: pool.ServerPool{},
		metricChan: monitoringChan,
	}
}

// balance forwards to any of the active backends the incoming request
func (lb *LoadBalancer) balance(w http.ResponseWriter, r *http.Request) {
	attempts := getAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := lb.serverPool.GetNextPeer()
	if peer != nil {
		requestTime := time.Now()
		peer.ReverseProxy.ServeHTTP(w, r)
		responseTime := time.Now()
		delta := responseTime.Sub(requestTime)
		lb.metricChan <- monitoring.Metric{
			Backend: peer,
			Value:   float64(delta.Milliseconds()),
		}
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

// AddServer adds a new backend to the server pool
func (lb *LoadBalancer) AddServer(serverURL *url.URL) {
	proxy := httputil.NewSingleHostReverseProxy(serverURL)
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
		log.Printf("[%s] %s\n", serverURL.Host, e.Error())
		retries := getRetryFromContext(request)
		if retries < 3 {
			select {
			case <-time.After(10 * time.Millisecond):
				ctx := context.WithValue(request.Context(), Retry, retries+1)
				proxy.ServeHTTP(writer, request.WithContext(ctx))
			}
			return
		}

		// after 3 retries, mark this backend as down
		lb.serverPool.StoreBackendStatus(serverURL, false)

		// if the same request routing for few attempts with different backends, increase the count
		attempts := getAttemptsFromContext(request)
		log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
		ctx := context.WithValue(request.Context(), Attempts, attempts+1)
		lb.balance(writer, request.WithContext(ctx))
	}

	lb.serverPool.AddBackend(&backend.Backend{
		URL:          serverURL,
		Alive:        true,
		ReverseProxy: proxy,
	})
}

// Run starts the load balancer at the specified port
func (lb *LoadBalancer) Run(port int) error {

	// create http server
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb.balance),
	}

	// start health checking
	go lb.healthCheck()

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

// getRetryFromContext returns the retries for request
func getRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

// getAttemptsFromContext returns the attempts for request
func getAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}
	return 1
}

// healthCheck runs a routine for check status of the backends every 20 secs
func (lb *LoadBalancer) healthCheck() {
	t := time.NewTicker(time.Second * 20)
	for {
		select {
		case <-t.C:
			log.Println("Starting health check...")
			lb.serverPool.HealthCheck()
			log.Println("Health check completed")
		}
	}
}

// Shutdown is called when the controller has finished its work
func (lb *LoadBalancer) Shutdown() {
	utilruntime.HandleCrash()
}
