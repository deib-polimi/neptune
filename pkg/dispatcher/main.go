package main

import (
	"flag"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer"
	lbcontroller "github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/controller"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring"
	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eainformers "github.com/lterrac/edge-autoscaler/pkg/generated/informers/externalversions"
	informers2 "github.com/lterrac/edge-autoscaler/pkg/informers"
	"github.com/lterrac/edge-autoscaler/pkg/signals"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	eaclient, err := eaclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubernetesClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	eaInformerFactory := eainformers.NewSharedInformerFactory(eaclient, time.Second*30)
	coreInformerFactory := informers.NewSharedInformerFactory(kubernetesClient, time.Second*30)

	// TODO: check name of this variable
	informers := informers2.Informers{
		ConfigMap:              coreInformerFactory.Core().V1().ConfigMaps(),
		Pod:                    coreInformerFactory.Core().V1().Pods(),
		Node:                   coreInformerFactory.Core().V1().Nodes(),
		Service:                coreInformerFactory.Core().V1().Services(),
		CommunitySchedule:      eaInformerFactory.Edgeautoscaler().V1alpha1().CommunitySchedules(),
		CommunityConfiguration: eaInformerFactory.Edgeautoscaler().V1alpha1().CommunityConfigurations(),
	}

	monitoringChan := make(chan monitoring.Metric)

	// TODO: retrieve backends from configmap
	_ = balancer.NewLoadBalancer(monitoringChan)

	coreInformerFactory.Start(stopCh)
	eaInformerFactory.Start(stopCh)

	lbController := lbcontroller.NewController(
		kubernetesClient,
		eaclient,
		informers,
	)

	if err = lbController.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running system controller: %s", err.Error())
	}
	defer lbController.Shutdown()

	<-stopCh
	klog.Info("Shutting down workers")
}

// rp := httputil.NewSingleHostReverseProxy(u)

// _ = http.HandlerFunc(rp.ServeHTTP)
// func main() {
// 	logs.InitLogs()
// 	defer logs.FlushLogs()
// 	stopCh := signals.SetupSignalHandler()

// 	cmd := &ResponseTimeMetricsAdapter{}

// 	cmd.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(apiserver.Scheme))
// 	cmd.OpenAPIConfig.Info.Title = "response-time-metrics-adapter"
// 	cmd.OpenAPIConfig.Info.Version = "0.1.0"

// 	cmd.Flags().AddGoFlagSet(flag.CommandLine) // make sure we get the klog flags
// 	cmd.Flags().Parse(os.Args)

// 	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
// 	if err != nil {
// 		klog.Fatalf("Error building kubeconfig: %s", err.Error())
// 	}

// 	saClient, err := clientset.NewForConfig(cfg)
// 	if err != nil {
// 		klog.Fatalf("Error building example clientset: %s", err.Error())
// 	}

// 	kubernetesClient, err := kubernetes.NewForConfig(cfg)
// 	if err != nil {
// 		klog.Fatalf("Error building example clientset: %s", err.Error())
// 	}

// 	saInformerFactory := sainformers.NewSharedInformerFactory(saClient, time.Second*30)
// 	coreInformerFactory := coreinformers.NewSharedInformerFactory(kubernetesClient, time.Second*30)

// 	// TODO: Check name of this variable
// 	informers := informers2.Informers{
// 		Pod:                   coreInformerFactory.Core().V1().Pods(),
// 		Node:                  coreInformerFactory.Core().V1().Nodes(),
// 		Service:               coreInformerFactory.Core().V1().Services(),
// 		PodScale:              saInformerFactory.Systemautoscaler().V1beta1().PodScales(),
// 		ServiceLevelAgreement: saInformerFactory.Systemautoscaler().V1beta1().ServiceLevelAgreements(),
// 	}

// 	coreInformerFactory.Start(stopCh)
// 	saInformerFactory.Start(stopCh)

// 	// TODO: handle this in a better way
// 	go informers.Pod.Informer().Run(stopCh)
// 	go informers.Node.Informer().Run(stopCh)
// 	go informers.Service.Informer().Run(stopCh)
// 	go informers.PodScale.Informer().Run(stopCh)
// 	go informers.ServiceLevelAgreement.Informer().Run(stopCh)

// 	if ok := cache.WaitForCacheSync(
// 		stopCh,
// 		informers.Pod.Informer().HasSynced,
// 		informers.PodScale.Informer().HasSynced,
// 		informers.Service.Informer().HasSynced,
// 		informers.ServiceLevelAgreement.Informer().HasSynced,
// 	); !ok {
// 		klog.Fatalf("failed to wait for caches to sync")
// 	}

// 	responseTimeMetricsProvider := cmd.makeProviderOrDie(informers, stopCh)
// 	cmd.WithCustomMetrics(responseTimeMetricsProvider)

// 	if err := cmd.Run(stopCh); err != nil {
// 		klog.Fatalf("unable to run custom metrics adapter: %v", err)
// 	}
// }

// --------------------------------------------------------------------------------------------------------------------------

// import (
// 	"fmt"
// 	"log"
// 	"math"
// 	"net/http"
// 	"net/http/httputil"
// 	"net/url"
// 	"os"
// 	"time"

// 	"github.com/asecurityteam/rolling"
// )

// var target = &url.URL{}
// var window = &rolling.TimePolicy{}

// // Environment
// var address string
// var port string
// var windowSize time.Duration
// var windowGranularity time.Duration

// func main() {
// 	mux := http.NewServeMux()

// 	mux.Handle("/metric/response_time", http.HandlerFunc(ResponseTime))
// 	mux.Handle("/metric/request_count", http.HandlerFunc(RequestCount))
// 	mux.Handle("/metric/throughput", http.HandlerFunc(Throughput))
// 	mux.Handle("/metrics/", http.HandlerFunc(AllMetrics))
// 	mux.Handle("/", http.HandlerFunc(ForwardRequest))

// 	address = os.Getenv("ADDRESS")
// 	port = os.Getenv("PORT")
// 	windowSizeString := os.Getenv("WINDOW_SIZE")
// 	windowGranularityString := os.Getenv("WINDOW_GRANULARITY")

// 	var err error
// 	log.Println("Reading environment variables")

// 	srv := &http.Server{
// 		Addr:    ":8000",
// 		Handler: mux,
// 	}
// 	target, _ = url.Parse("http://" + address + ":" + port)
// 	log.Println("Forwarding all requests to:", target)

// 	windowSize, err = time.ParseDuration(windowSizeString)

// 	if err != nil {
// 		log.Fatalf("Failed to parse windows size. Error: %v", err)
// 	}

// 	windowGranularity, err = time.ParseDuration(windowGranularityString)

// 	if err != nil {
// 		log.Fatalf("Failed to parse windows granularity. Error: %v", err)
// 	}

// 	window = rolling.NewTimePolicy(rolling.NewWindow(int(windowSize.Nanoseconds()/windowGranularity.Nanoseconds())), time.Millisecond)
// 	log.Println("Time window initialized with size:", windowSizeString, " and granularity:", windowGranularityString)

// 	// output error and quit if ListenAndServe fails
// 	log.Fatal(srv.ListenAndServe())

// }

// // ForwardRequest send all the request the the pod except for the ones having metrics/ in the path
// func ForwardRequest(res http.ResponseWriter, req *http.Request) {
// 	requestTime := time.Now()
// 	httputil.NewSingleHostReverseProxy(target).ServeHTTP(res, req)
// 	responseTime := time.Now()
// 	delta := responseTime.Sub(requestTime)
// 	window.Append(float64(delta.Milliseconds()))
// }

// // ResponseTime return the pod average response time
// func ResponseTime(res http.ResponseWriter, req *http.Request) {
// 	responseTime := window.Reduce(rolling.Avg)
// 	if math.IsNaN(responseTime) {
// 		responseTime = 0
// 	}
// 	_, _ = fmt.Fprintf(res, `{"%s": %f}`, metrics.ResponseTime.String(), responseTime)
// }

// // RequestCount return the current number of request sent to the pod
// func RequestCount(res http.ResponseWriter, req *http.Request) {
// 	requestCount := window.Reduce(rolling.Count)
// 	if math.IsNaN(requestCount) {
// 		requestCount = 0
// 	}
// 	_, _ = fmt.Fprintf(res, `{"%s": %f}`, metrics.RequestCount.String(), requestCount)
// }

// // Throughput returns the pod throughput in request per second
// func Throughput(res http.ResponseWriter, req *http.Request) {
// 	throughput := window.Reduce(rolling.Count) / windowSize.Seconds()
// 	_, _ = fmt.Fprintf(res, `{"%s": %f}`, metrics.Throughput.String(), throughput)
// }

// // AllMetrics returns all the metrics available for the pod
// func AllMetrics(res http.ResponseWriter, req *http.Request) {
// 	responseTime := window.Reduce(rolling.Avg)
// 	if math.IsNaN(responseTime) {
// 		responseTime = 0
// 	}

// 	requestCount := window.Reduce(rolling.Count)
// 	if math.IsNaN(requestCount) {
// 		requestCount = 0
// 	}

// 	throughput := window.Reduce(rolling.Count) / windowSize.Seconds()
// 	// TODO: maybe we should wrap this into an helper function of metrics struct
// 	_, _ = fmt.Fprintf(res, `{"%s": %f,"%s": %f,"%s": %f}`, metrics.ResponseTime.String(), responseTime, metrics.RequestCount.String(), requestCount, metrics.Throughput.String(), throughput)
// }
