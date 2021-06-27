package controller

import (
	"fmt"
	"time"

	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eascheme "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned/scheme"
	"github.com/lterrac/edge-autoscaler/pkg/informers"
	"github.com/lterrac/edge-autoscaler/pkg/queue"
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
	controllerAgentName = "system-controller"
)

type SystemController struct {
	// saClientSet is a clientset for our own API group
	edgeAutoscalerClientSet eaclientset.Interface

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Interface

	listers informers.Listers

	nodeSynced              cache.InformerSynced
	communitySettingsSynced cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// workqueue contains all the servicelevelagreements that needs a recommendation
	workqueue queue.Queue
}

func NewController(
	kubernetesClientset *kubernetes.Clientset,
	eaClientSet eaclientset.Interface,
	informers informers.Informers,
) *SystemController {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(eascheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	// Instantiate the Controller
	controller := &SystemController{
		edgeAutoscalerClientSet: eaClientSet,
		kubernetesClientset:     kubernetesClientset,
		recorder:                recorder,
		listers:                 informers.GetListers(),
		nodeSynced:              informers.Node.Informer().HasSynced,
		communitySettingsSynced: informers.CommunitySettingses.Informer().HasSynced,
		workqueue:               queue.NewQueue("CommunitySettingsQueue"),
	}

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *SystemController) Run(threadiness int, stopCh <-chan struct{}) error {

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting system level controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(
		stopCh,
		c.communitySettingsSynced,
		c.nodeSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting system controller workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	return nil
}

func (c *SystemController) runWorker() {
	for c.workqueue.ProcessNextItem(c.handleCommunitySettings) {
	}
}

func (c *SystemController) handleCommunitySettings(key string) error {

	return nil
}

// Shutdown is called when the controller has finished its work
func (c *SystemController) Shutdown() {
	utilruntime.HandleCrash()
}
