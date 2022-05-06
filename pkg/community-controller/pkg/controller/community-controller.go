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
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	controllerAgentName string = "community-controller"

	// SuccessSynced is used as part of the Event 'reason' when a podScale is synced
	SuccessSynced string = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a podScale
	// is synced successfully
	MessageResourceSynced string = "Community Settings synced successfully"
)

// CommunityController schedules the pod in a community and ensures that pod allocations defined in
// the community schedule is attuated
type CommunityController struct {
	// saClientSet is a clientset for our own API group
	edgeAutoscalerClientSet eaclientset.Interface

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Interface

	listers informers.Listers

	nodeSynced                    cache.InformerSynced
	communityConfigurationsSynced cache.InformerSynced
	communitySchedulesSynced      cache.InformerSynced
	functionSynced                cache.InformerSynced
	podsSynced                    cache.InformerSynced

	communityName      string
	communityNamespace string

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// syncCommunityScheduleWorkqueue contains the community schedules which have to be synced
	syncCommunityScheduleWorkqueue queue.Queue
}

// NewController returns a new CommunityController
func NewController(
	kubernetesClientset *kubernetes.Clientset,
	eaClientSet eaclientset.Interface,
	informers informers.Informers,
	communityNamespace string,
	communityName string,
) *CommunityController {

	// Create event broadcaster
	// Add system-controller types to the default Kubernetes Scheme so Events can be
	// logged for system-controller types.
	utilruntime.Must(eascheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	// Instantiate the Controller
	controller := &CommunityController{
		edgeAutoscalerClientSet:        eaClientSet,
		kubernetesClientset:            kubernetesClientset,
		recorder:                       recorder,
		listers:                        informers.GetListers(),
		nodeSynced:                     informers.Node.Informer().HasSynced,
		communityConfigurationsSynced:  informers.CommunityConfiguration.Informer().HasSynced,
		communitySchedulesSynced:       informers.CommunitySchedule.Informer().HasSynced,
		functionSynced:                 informers.Function.Informer().HasSynced,
		podsSynced:                     informers.Pod.Informer().HasSynced,
		syncCommunityScheduleWorkqueue: queue.NewQueue("SyncCommunityScheduleWorkqueue", workqueue.NewItemExponentialFailureRateLimiter(10*time.Millisecond, 5*time.Second)),
		communityName:                  communityName,
		communityNamespace:             communityNamespace,
	}

	klog.Info("Setting up event handlers")
	informers.CommunitySchedule.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleCommunitySchedule,
		DeleteFunc: controller.handleCommunitySchedule,
		UpdateFunc: controller.handleCommunityScheduleUpdate,
	})
	informers.Pod.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handlePod,
		DeleteFunc: controller.handlePod,
		UpdateFunc: controller.handlePodUpdate,
	})
	informers.Node.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleNode,
		DeleteFunc: controller.handleNode,
		UpdateFunc: controller.handleNodeUpdate,
	})
	informers.Function.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleFunction,
		DeleteFunc: controller.handleFunction,
		UpdateFunc: controller.handleFunctionUpdate,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *CommunityController) Run(threadiness int, stopCh <-chan struct{}) error {

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting system level controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(
		stopCh,
		c.communityConfigurationsSynced,
		c.communitySchedulesSynced,
		c.nodeSynced,
		c.functionSynced,
		c.podsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Infof("Starting community controller for community %s/%s", c.communityNamespace, c.communityName)
	klog.Info("Starting system controller workers")

	for i := 0; i < threadiness; i++ {
		// TODO: Currently the scheduler reschedules pods every 30 seconds. It should be change to be triggered by event or as cron jobs
		go wait.Until(c.runPeriodicScheduleWorker, 180*time.Second, stopCh)
		go wait.Until(c.runSyncCommunitySchedule, time.Second, stopCh)
	}

	return nil
}

// runPeriodicScheduleWorker is a worker which runs the scheduling algorithm
func (c *CommunityController) runPeriodicScheduleWorker() {
	_ = c.runScheduler("")
}

// runSyncCommunitySchedule is a worker which looks for inconsistencies between
// the community schedule and current allocation of pods.
// If any are found, pods are created or deleted.
func (c *CommunityController) runSyncCommunitySchedule() {
	for c.syncCommunityScheduleWorkqueue.ProcessNextItem(c.syncCommunityScheduleAllocation) {
	}
}

// Shutdown is called when the controller has finished its work
func (c *CommunityController) Shutdown() {
	utilruntime.HandleCrash()
}
