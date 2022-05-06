package slpaclient

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"k8s.io/klog/v2"
)

const (
	// Communities is the path that starts SLPA algorithm
	Communities Path = "/api/communities"
)

// Path represents the REST paths offered by the SLPA microservice
type Path string

// ClientCommunityGetter wraps CommunityGettere interface and offers a
// function to set the host
type ClientCommunityGetter interface {
	CommunityGetter
	SetHost(host string)
}

// CommunityGetter is the standard interface to retrieve communities
type CommunityGetter interface {
	Communities(req *RequestSLPA) ([]Community, error)
}

// Client is used to interact with SLPA algorithm
type Client struct {
	Host       string
	httpClient http.Client
}

func (p Path) string() string {
	return string(p)
}

// NewClient returns a new MetricClient representing a metric client.
func NewClient() *Client {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 90 * time.Second,
			}).DialContext,
			MaxIdleConns:          50,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
		},
		Timeout: 20 * time.Second,
	}
	client := &Client{
		httpClient: httpClient,
	}
	return client
}

// SetHost set the hosst
func (c *Client) SetHost(host string) {
	c.Host = host
}

// Communities returns the results of SLPA algorithm
func (c *Client) Communities(req *RequestSLPA) ([]Community, error) {

	communities, err := c.sendRequest(req, Communities)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return c.parseRawCommunities(communities)
}

func (c *Client) parseRawCommunities(communities []byte) ([]Community, error) {
	var res ResponseSLPA
	err := json.Unmarshal(communities, &res)

	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return res.Communities, nil
}

func (c *Client) sendRequest(req *RequestSLPA, p Path) ([]byte, error) {

	path := p.string()

	//Create the request
	slpaServerURL := url.URL{
		Scheme: "http",
		Host:   c.Host,
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
