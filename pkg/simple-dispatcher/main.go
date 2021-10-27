package main

import (
	"fmt"
	"github.com/lterrac/edge-autoscaler/pkg/db"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/persistor"
	"github.com/lterrac/edge-autoscaler/pkg/metrics"
	"io"
	"k8s.io/klog/v2"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/asecurityteam/rolling"
)

var target = &url.URL{}
var window = &rolling.TimePolicy{}

// Environment
var address string
var port string
var windowSize time.Duration
var windowGranularity time.Duration

// MetricsDB
var database *persistor.MetricsPersistor
var metricChan chan metrics.RawResponseTime

// Useful information
var node string
var function string
var community string
var namespace string
var gpu bool

// Proxy client
var client *http.Client

func main() {

	var err error

	node = getenv("NODE", "")
	function = getenv("FUNCTION", "")
	community = getenv("COMMUNITY", "")
	namespace = getenv("NAMESPACE", "")
	gpu, err = strconv.ParseBool(getenv("GPU", "false"))

	mux := http.NewServeMux()

	mux.Handle("/", http.HandlerFunc(ForwardRequest))

	address = getenv("ADDRESS", "localhost")
	port = getenv("APP_PORT", "8080")

	if err != nil {
		klog.Fatal("failed to parse GPU environment variable. It should be a boolean")
	}


	client = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				KeepAlive: 5 * time.Minute,
				Timeout:   90 * time.Second,
			}).DialContext,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
		},
		Timeout: time.Second * 30,
	}

	metricChan = make(chan metrics.RawResponseTime, 10000)

	database = persistor.NewMetricsPersistor(db.NewDBOptions(), metricChan)
	err = database.SetupDBConnection()
	if err != nil {
		klog.Fatal(err)
	}
	go database.PollMetrics()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// output error and quit if ListenAndServe fails
	log.Fatal(srv.ListenAndServe())

}

// ForwardRequest send all the request the the pod except for the ones having metrics/ in the path
func ForwardRequest(w http.ResponseWriter, req *http.Request) {
	requestTime := time.Now()

	klog.Infof("Received request : %v", req)

	paths := strings.SplitN(req.URL.Path, "/", 5)
	functionNamespace := paths[2]
	functionName := paths[3]
	functionPath := paths[4]
	klog.Info(functionPath)

	targetString := fmt.Sprintf("http://%s.%s.svc.cluster.local/%s", functionName, functionNamespace, functionPath)
	target, _ := url.Parse(targetString)
	newRequest := newRequest(req, target)

	res, err := client.Do(newRequest)
	if err != nil {
		klog.Error(err)
	}
	klog.Info(res)

	if res.Body != nil {
		defer res.Body.Close()
	}

	copyHeaders(w.Header(), &res.Header)

	w.WriteHeader(res.StatusCode)

	if res.Body != nil {
		// Copy the body over
		_, err = io.CopyBuffer(w, res.Body, nil)
	}

	//proxy := httputil.NewSingleHostReverseProxy(target)
	//proxy.ModifyResponse = logResponse
	//proxy.ServeHTTP(res, newRequest)
	responseTime := time.Now()
	delta := responseTime.Sub(requestTime)
	metricChan <- metrics.RawResponseTime{
		Timestamp:   time.Now(),
		Function:    function,
		Source:      node,
		Destination: "not-defined",
		Namespace:   namespace,
		Community:   community,
		Gpu:         gpu,
		Latency:     int(delta.Milliseconds()),
	}

}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		klog.Warningf("failed parsing environment variable %s, setting it to default value %s", key, fallback)
		return fallback
	}
	klog.Infof("parsed environment variable %s with value %s", key, value)
	return value
}

func newRequest(old *http.Request, target *url.URL) *http.Request {

	req, err := http.NewRequest(old.Method, target.String(), old.Body)

	if err != nil {
		klog.Error(err)
	}

	copyHeaders(req.Header, &old.Header)

	if len(old.Host) > 0 && req.Header.Get("X-Forwarded-Host") == "" {
		req.Header["X-Forwarded-Host"] = []string{old.Host}
	}

	if req.Header.Get("X-Forwarded-For") == "" {
		req.Header["X-Forwarded-For"] = []string{old.RemoteAddr}
	}

	if old.Body != nil {
		req.Body = old.Body
	}

	return req
}

func copyHeaders(destination http.Header, source *http.Header) {
	for k, v := range *source {
		vClone := make([]string, len(v))
		copy(vClone, v)
		(destination)[k] = vClone
	}
}
