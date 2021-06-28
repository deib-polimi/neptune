package slpaclient

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// Client is used to interact with SLPA algorithm
type Client struct {
	Host       string
	httpClient http.Client
}

// Path represents the REST paths offered by the SLPA microservice
type Path string

const (
	// Communities is the path that starts SLPA algorithm
	Communities Path = "/api/communities"
)

func (p Path) String() string {
	return string(p)
}

// NewClient returns a new MetricClient representing a metric client.
func NewClient() *Client {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 90 * time.Second,
			}).DialContext,
			// TODO: Some of those value should be tuned
			MaxIdleConns:          50,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
		},
		Timeout: 20 * time.Second,
	}
	client := &Client{
		httpClient: httpClient,
		Host:       "{pod_address}:{pod_port}",
	}
	return client
}

// Communities returns the results of SLPA algorithm
func (c *Client) Communities(req *RequestSLPA) ([]Community, error) {
	communities, err := c.sendRequest(req, Communities)

	var res ResponseSLPA
	err = json.Unmarshal(communities, &res)

	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return res.Communities, nil
}

func (c *Client) sendRequest(req *RequestSLPA, p Path) ([]byte, error) {
	//TODO: make it custom
	slpaAddress := "127.0.0.1"

	// Compose host and path
	host := c.Host
	host = strings.Replace(host, "{pod_address}", slpaAddress, -1)
	host = strings.Replace(host, "{pod_port}", "4567", -1)
	path := p.String()

	// Create the request
	slpaServerURL := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   path,
	}

	body, err := json.Marshal(req)

	if err != nil {
		klog.Error(err)
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, slpaServerURL.String(), bytes.NewBuffer(body))

	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// Send the request
	response, err := c.httpClient.Do(request)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// Parse the response
	return ioutil.ReadAll(response.Body)
}
