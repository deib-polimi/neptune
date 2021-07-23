package controller

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/queue"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring"
	monitoringmetrics "github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eascheme "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned/scheme"
	"github.com/lterrac/edge-autoscaler/pkg/informers"
	workqueue "github.com/lterrac/edge-autoscaler/pkg/queue"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

const (
	controllerAgentName string = "loadbalancer-controller"

	// SuccessSynced is used as part of the Event 'reason' when a podScale is synced
	SuccessSynced string = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a configmap
	// is synced successfully
	MessageResourceSynced string = "Community Settings synced successfully"
)

// LoadBalancerController works at node level to forward an incoming request for a function
// to the right backend, implementing load balancing policies.
type LoadBalancerController struct {
	// saClientSet is a clientset for our own API group
	edgeAutoscalerClientSet eaclientset.Interface

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Interface

	// balancers keeps track of the load balancers associated to a function
	// function name is the key
	balancers map[string]*balancer.LoadBalancer

	listers informers.Listers

	nodeSynced cache.InformerSynced

	communityScheduleSynced cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// workqueue contains all the communityconfigurations to sync
	workqueue workqueue.Queue

	requestqueue *queue.RequestQueue

	serverListener http.Server

	monitoringChan chan<- monitoringmetrics.RawMetricData

	backendChan chan<- monitoring.BackendList

	// node is the node name on which the controller is running
	node string
}

// NewController returns a new SystemController
func NewController(
	kubernetesClientset *kubernetes.Clientset,
	eaClientSet eaclientset.Interface,
	informers informers.Informers,
	monitoringChan chan<- monitoringmetrics.RawMetricData,
	node string,
) *LoadBalancerController {

	// Create event broadcaster
	// Add system-controller types to the default Kubernetes Scheme so Events can be
	// logged for system-controller types.
	utilruntime.Must(eascheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	// Instantiate the Controller
	controller := &LoadBalancerController{
		edgeAutoscalerClientSet: eaClientSet,
		kubernetesClientset:     kubernetesClientset,
		recorder:                recorder,
		listers:                 informers.GetListers(),
		nodeSynced:              informers.Node.Informer().HasSynced,
		communityScheduleSynced: informers.CommunitySchedule.Informer().HasSynced,
		workqueue:               workqueue.NewQueue("ConfigMapQueue"),
		requestqueue:            queue.NewRequestQueue(),
		monitoringChan:          monitoringChan,
		balancers:               make(map[string]*balancer.LoadBalancer),
		node:                    node,
	}

	klog.Info("Setting up event handlers")

	//TODO: should we use event handlers or simply periodically poll the new resource?
	// Set up an event handler for when CommunitySchedule resources change
	informers.CommunitySchedule.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.workqueue.Add,
		UpdateFunc: controller.workqueue.Update,
		DeleteFunc: controller.workqueue.Deletion,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *LoadBalancerController) Run(threadiness int, stopCh <-chan struct{}) error {

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting load balancer controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(
		stopCh,
		//TODO: change with proper CRD
		c.communityScheduleSynced,
		c.nodeSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting system controller workers")

	//TODO: set port with env var
	//Listen for incoming request
	go c.listenAndServe(80)

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runStandardWorker, time.Second, stopCh)
	}

	for i := 0; i < threadiness; i++ {
		go c.dispatchRequest(stopCh)
	}

	return nil
}

func (c *LoadBalancerController) listenAndServe(port int) {
	// create http server
	c.serverListener = http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(c.enqueueRequest),
	}

	klog.Info("server listener started at :%d\n", port)

	err := c.serverListener.ListenAndServe()

	utilruntime.HandleError(fmt.Errorf("closing server listener: %s", err))
}

// handles standard partitioning (e.g. first partitioning and cache sync)
func (c *LoadBalancerController) runStandardWorker() {
	for c.workqueue.ProcessNextItem(c.syncCommunitySchedule) {
	}
}

func (c *LoadBalancerController) enqueueRequest(w http.ResponseWriter, r *http.Request) {
	// TODO: a better way would be to check for openfaas-gateway
	if strings.Contains(r.URL.Host, "/function/") {
		c.requestqueue.Enqueue(&queue.HTTPRequest{
			ResponseWriter: w,
			Request:        r,
		})

		return
	}

	// forward any other request
	go httputil.NewSingleHostReverseProxy(r.URL).ServeHTTP(w, r)

}

func (c *LoadBalancerController) dispatchRequest(stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		req := c.requestqueue.Dequeue()

		if req == nil {
			continue
		}

		// TODO: get correct function name from request
		if balancer, exist := c.balancers[functionName(req.Request.URL)]; exist {
			balancer.Balance(req.ResponseWriter, req.Request)
		}
	}
}

// Shutdown is called when the controller has finished its work
func (c *LoadBalancerController) Shutdown() {
	utilruntime.HandleCrash()
}

func functionName(url *url.URL) string {
	fragments := strings.Split(url.Host, "/")
	var index = 0
	for index = range fragments {
		if fragments[index] == "function" {
			break
		}
	}
	return fragments[index+1]
}
