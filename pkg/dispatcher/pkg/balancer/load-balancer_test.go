package balancer

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/queue"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

type FakeResponseWriter struct {
	header http.Header
	body   []byte
}

func (f *FakeResponseWriter) Header() http.Header {
	return f.header
}
func (f *FakeResponseWriter) Write(b []byte) (int, error) {
	f.body = b
	return len(b), nil
}
func (f *FakeResponseWriter) WriteHeader(statusCode int) {
	f.header.Set("code", fmt.Sprint(statusCode))
}

// always return 200
func healthyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func fakePoll(c <-chan metrics.RawResponseTime) {
	for {
		select {
		case <-c:
		default:
		}
	}
}

func noRecovery(req *queue.HTTPRequest) {}

func TestReverseProxy(t *testing.T) {
	testacases := []struct {
		description    string
		backends       []*url.URL
		createBackends func(url *url.URL) *http.Server
	}{
		{
			description: "Test balancer redirection",
			backends: []*url.URL{
				{
					Host:   "127.0.0.1:8081",
					Scheme: "http",
				},
			},
			createBackends: func(url *url.URL) *http.Server {
				// create http server
				server := &http.Server{
					Addr:    url.Host,
					Handler: http.HandlerFunc(healthyHandler),
				}

				return server
			},
		},
	}

	for _, tt := range testacases {
		t.Run(tt.description, func(t *testing.T) {
			servers := []*http.Server{}
			for _, url := range tt.backends {
				server := tt.createBackends(url)
				servers = append(servers, server)
				go server.ListenAndServe()
			}

			monitoringChan := make(chan metrics.RawResponseTime)
			go fakePoll(monitoringChan)

			lb := NewLoadBalancer(NodeInfo{}, monitoringChan)
			// lb := NewLoadBalancer(monitoringChan)

			for _, backend := range tt.backends {
				lb.AddServer(backend, "", false, resource.NewMilliQuantity(2, resource.BinarySI), noRecovery)
			}

			req := &http.Request{
				URL: &url.URL{
					Host:   "127.0.0.1:8081",
					Scheme: "http",
				},
			}
			response := &FakeResponseWriter{
				header: http.Header{},
				body:   make([]byte, 0),
			}

			lb.Balance(response, req)

			require.Equal(t, fmt.Sprint(http.StatusOK), response.header.Get("code"))
		})
	}
}
