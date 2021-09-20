package balancer

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/pool"
	"k8s.io/klog/v2"
)

// UpstreamRequestBuilder transforms the incoming requests URL in the one
type UpstreamRequestBuilder struct {
	// Request is the original http request
	Request *http.Request
	// BaseURL is the URL used by the client to connect to call a function outside the cluster.
	Backend pool.Backend
}

func (u UpstreamRequestBuilder) baseURL() string {
	return fmt.Sprintf("%v://%v", u.Backend.URL.Scheme, u.Backend.URL.Host)
}

func (u UpstreamRequestBuilder) buildRequestURL() string {
	parts := strings.Split(u.Request.URL.Path, "/")

	var prefixIndex int
	found := false
	for prefixIndex = range parts {
		if parts[prefixIndex] == "function" {
			found = true
			break
		}
	}

	if !found {
		return u.Request.URL.Path
	}

	gatewayPrefix := fmt.Sprintf("/%s/%s/%s", parts[prefixIndex], parts[prefixIndex+1], parts[prefixIndex+2])

	return strings.ReplaceAll(u.Request.URL.Path, gatewayPrefix, "")
}

func (u UpstreamRequestBuilder) Build() *http.Request {
	url := u.baseURL() + u.buildRequestURL()

	if len(u.Request.URL.RawQuery) > 0 {
		url = fmt.Sprintf("%s?%s", url, u.Request.URL.RawQuery)
	}

	upstreamReq, err := http.NewRequest(u.Request.Method, url, nil)
	upstreamReq.URL.Scheme = u.Backend.URL.Scheme

	if err != nil {
		klog.Errorf("Error creating upstream request: %v", err)
		return nil
	}

	copyHeaders(upstreamReq.Header, &u.Request.Header)

	if len(u.Request.Host) > 0 && upstreamReq.Header.Get("X-Forwarded-Host") == "" {
		upstreamReq.Header["X-Forwarded-Host"] = []string{u.Request.Host}
	}

	if upstreamReq.Header.Get("X-Forwarded-For") == "" {
		upstreamReq.Header["X-Forwarded-For"] = []string{u.Request.RemoteAddr}
	}

	if u.Request.Body != nil {
		upstreamReq.Body = u.Request.Body
	}

	return upstreamReq
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
