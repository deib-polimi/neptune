package balancer

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/pool"
	"github.com/stretchr/testify/require"
)

func TestBuilUpstreamRequest(t *testing.T) {
	testcases := []struct {
		description string
		request     *http.Request
		backend     pool.Backend
		expected    *http.Request
	}{
		{
			description: "change host",
			request: &http.Request{
				Proto:  "HTTP/1.1",
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "http",
					Host:   "example.com",
				},
				Host: "example.com",
			},
			backend: pool.Backend{
				URL: &url.URL{
					Scheme: "http",
					Host:   "proxy.org",
				},
			},
			expected: &http.Request{
				Proto:  "HTTP/1.1",
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "http",
					Host:   "proxy.org",
				},
				Host: "proxy.org",
				Header: map[string][]string{
					"X-Forwarded-For":  {""},
					"X-Forwarded-Host": {"example.com"},
				},
			},
		},
		{
			description: "delete function prefix",
			request: &http.Request{
				Proto:  "HTTP/1.1",
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "http",
					Host:   "example.com",
					Path:   "/function/foo/bar/",
				},
				Host: "example.com",
			},
			backend: pool.Backend{
				URL: &url.URL{
					Scheme: "http",
					Host:   "proxy.org",
				},
			},
			expected: &http.Request{
				Proto:  "HTTP/1.1",
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "http",
					Host:   "proxy.org",
					Path:   "/",
				},
				Host: "proxy.org",
				Header: map[string][]string{
					"X-Forwarded-For":  {""},
					"X-Forwarded-Host": {"example.com"},
				},
			},
		},
		{
			description: "maintain function path",
			request: &http.Request{
				Proto:  "HTTP/1.1",
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "http",
					Host:   "example.com",
					Path:   "/function/foo/bar/a",
				},
				Host: "example.com",
			},
			backend: pool.Backend{
				URL: &url.URL{
					Scheme: "http",
					Host:   "proxy.org",
				},
			},
			expected: &http.Request{
				Proto:  "HTTP/1.1",
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "http",
					Host:   "proxy.org",
					Path:   "/a",
				},
				Host: "proxy.org",
				Header: map[string][]string{
					"X-Forwarded-For":  {""},
					"X-Forwarded-Host": {"example.com"},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			u := &UpstreamRequestBuilder{
				Request: tt.request,
				Backend: tt.backend,
			}

			actual := u.Build()

			require.Equal(t, tt.expected.Method, actual.Method)
			require.Equal(t, tt.expected.Proto, actual.Proto)
			require.Equal(t, tt.expected.Header, actual.Header)
			require.Equal(t, tt.expected.Body, actual.Body)
			require.Equal(t, tt.expected.Host, actual.Host)
			require.Equal(t, tt.expected.URL, actual.URL)
		})
	}
}
