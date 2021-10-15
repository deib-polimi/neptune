package controller

import (
	"fmt"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/delayclient"

	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eascheme "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned/scheme"
	"github.com/lterrac/edge-autoscaler/pkg/informers"
	"github.com/lterrac/edge-autoscaler/pkg/queue"
	slpaClient "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
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
	controllerAgentName string = "system-controller"

	// SuccessSynced is used as part of the Event 'reason' when a podScale is synced
	SuccessSynced string = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a community configuration
	// is synced successfully
	MessageResourceSynced string = "Community Settings synced successfully"
)

// SystemController works at cluster level to divide the computational resources
// in communities. It is also responsible of modify the communities according to
// changes in cluster topology and in case of performance degradation
type SystemController struct {
	// saClientSet is a clientset for our own API group
	edgeAutoscalerClientSet eaclientset.Interface

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Interface

	// slpaClient is used to interact with SLPA algorithm
	communityGetter slpaClient.ClientCommunityGetter

	// communityUpdater applies the output of SLPA to Kubernets Nodes
	communityUpdater *CommunityUpdater

	// delayClient retrieves the delay matrix
	delayClient delayclient.DelayClient

	listers informers.Listers

	nodeSynced                    cache.InformerSynced
	deploymentSynced              cache.InformerSynced
	communityScheduleSynced       cache.InformerSynced
	communityConfigurationsSynced cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// syncConfigurationsWorkqueue contains all the communityconfigurations to sync
	syncConfigurationsWorkqueue queue.Queue
	// syncConfigurationsWorkqueue contains all the communityschedules to sync
	syncSchedulesWorkqueue queue.Queue
}

// NewController returns a new SystemController
func NewController(
	kubernetesClientset *kubernetes.Clientset,
	eaClientSet eaclientset.Interface,
	informers informers.Informers,
	communityUpdater *CommunityUpdater,
	communityGetter slpaClient.ClientCommunityGetter,
	delayClient delayclient.DelayClient,
) *SystemController {

	// Create event broadcaster
	// Add system-controller types to the default Kubernetes Scheme so Events can be
	// logged for system-controller types.
	utilruntime.Must(eascheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	// Instantiate the Controller
	controller := &SystemController{
		edgeAutoscalerClientSet:       eaClientSet,
		kubernetesClientset:           kubernetesClientset,
		communityUpdater:              communityUpdater,
		communityGetter:               communityGetter,
		delayClient:                   delayClient,
		recorder:                      recorder,
		listers:                       informers.GetListers(),
		deploymentSynced:              informers.Deployment.Informer().HasSynced,
		communityScheduleSynced:       informers.CommunitySchedule.Informer().HasSynced,
		nodeSynced:                    informers.Node.Informer().HasSynced,
		communityConfigurationsSynced: informers.CommunityConfiguration.Informer().HasSynced,
		syncConfigurationsWorkqueue:   queue.NewQueue("CommunityConfigurationsQueue", nil),
		syncSchedulesWorkqueue:        queue.NewQueue("CommunityScheduleQueue", nil),
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when ServiceLevelAgreements resources change
	informers.CommunityConfiguration.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleCommunityConfigurationsAdd,
		UpdateFunc: controller.handleCommunityConfigurationsUpdate,
		DeleteFunc: controller.handleCommunityConfigurationsDeletion,
	})

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
		c.communityConfigurationsSynced,
		c.nodeSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting system controller workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runStandardWorker, time.Second, stopCh)
		go wait.Until(c.runSyncSchedulesWorker, time.Second, stopCh)
	}

	// TODO: implement
	// go wait.Until(c.runPerformanceDegradationObserver, time.Second, stopCh)
	// go wait.Until(c.runTopologyObserver, time.Second, stopCh)

	return nil
}

// handles standard partitioning (e.g. first partioning and cache sync)
func (c *SystemController) runStandardWorker() {
	for c.syncConfigurationsWorkqueue.ProcessNextItem(c.syncCommunityConfiguration) {
	}
}

// handles standard partitioning (e.g. first partioning and cache sync)
func (c *SystemController) runSyncSchedulesWorker() {
	for c.syncSchedulesWorkqueue.ProcessNextItem(c.syncCommunitySchedules) {
	}
}

// Shutdown is called when the controller has finished its work
func (c *SystemController) Shutdown() {
	utilruntime.HandleCrash()
}

// TODO: add deployment and configuration schedule handlers in order to insta add or delete unnecessary things
