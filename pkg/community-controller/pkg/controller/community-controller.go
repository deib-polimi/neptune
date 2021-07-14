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

	communityName      string
	communityNamespace string
	configurationName  string

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// workqueue contains all the communityconfigurations to sync
	deploymentWorkqueue queue.Queue
	// workqueue contains all the communityconfigurations to sync
	schedulerWorkqueue queue.Queue
	// workqueue contains all the communityconfigurations to sync
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
		deploymentWorkqueue:            queue.NewQueue("DeploymentsQueue"),
		schedulerWorkqueue:             queue.NewQueue("SchedulerQueue"),
		syncCommunityScheduleWorkqueue: queue.NewQueue("SyncCommunityScheduleWorkequeue"),
		communityName:                  communityName,
		communityNamespace:             communityNamespace,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when ServiceLevelAgreements resources change
	informers.Deployment.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleDeploymentAdd,
		UpdateFunc: controller.handleDeploymentUpdate,
		DeleteFunc: controller.handleDeploymentDelete,
	})
	informers.CommunityConfiguration.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleCommunityConfigurationAdd,
		UpdateFunc: controller.handleCommunityConfigurationUpdate,
		DeleteFunc: controller.handleCommunityConfigurationDelete,
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
		c.nodeSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting system controller workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runDeploymentReplicasSyncWorker, time.Second, stopCh)
		//go wait.Until(c.runScheduleWorker, time.Second, stopCh)
		//go wait.Until(c.runPeriodicScheduleWorker, 30*time.Second, stopCh)
		//go wait.Until(c.runSchedulePodsWorker, 30*time.Second, stopCh)
		go wait.Until(c.runSyncCommunitySchedule, time.Second, stopCh)
	}

	// TODO: implement
	// go wait.Until(c.runPerformanceDegradationObserver, time.Second, stopCh)
	// go wait.Until(c.runTopologyObserver, time.Second, stopCh)

	return nil
}

// TODO: add comment
func (c *CommunityController) runDeploymentReplicasSyncWorker() {
	for c.deploymentWorkqueue.ProcessNextItem(c.syncDeploymentReplicas) {
	}
}

// TODO: add comment
func (c *CommunityController) runScheduleWorker() {
	for c.schedulerWorkqueue.ProcessNextItem(c.runScheduler) {
	}
}

// TODO: add comment
func (c *CommunityController) runPeriodicScheduleWorker() {
	err := c.runScheduler("wow")
	klog.Info(err)
}

// TODO: add comment
func (c *CommunityController) runSchedulePodsWorker() {
	err := c.schedulePods(fmt.Sprintf("%s/%s", c.communityNamespace, c.communityName))
	klog.Info(err)
}

// TODO: add comment
func (c *CommunityController) runSyncCommunitySchedule() {
	for c.syncCommunityScheduleWorkqueue.ProcessNextItem(c.syncCommunitySchedule) {
	}
}

// Shutdown is called when the controller has finished its work
func (c *CommunityController) Shutdown() {
	utilruntime.HandleCrash()
}
