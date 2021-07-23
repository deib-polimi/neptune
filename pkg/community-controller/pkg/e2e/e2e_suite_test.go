package e2e_test

import (
	"context"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaasclientsent "github.com/openfaas/faas-netes/pkg/client/clientset/versioned"
	openfaasinformers "github.com/openfaas/faas-netes/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"testing"
	"time"

	comcontroller "github.com/lterrac/edge-autoscaler/pkg/community-controller/pkg/controller"
	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eascheme "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned/scheme"
	eainformers "github.com/lterrac/edge-autoscaler/pkg/generated/informers/externalversions"
	"github.com/lterrac/edge-autoscaler/pkg/informers"
	"github.com/lterrac/edge-autoscaler/pkg/signals"
	coreinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/gomega"

	. "github.com/onsi/ginkgo"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}

var cfg *rest.Config
var testEnv *envtest.Environment
var kubeClient *kubernetes.Clientset
var eaClient *eaclientset.Clientset
var openfaasClient *openfaasclientsent.Clientset
var communityController *comcontroller.CommunityController
var workerNodes []*corev1.Node

const communityNamespace = "e2e"
const communityName = "test-com1"

var _ = BeforeSuite(func() {
	done := make(chan interface{})
	useCluster := true

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster:       &useCluster,
		AttachControlPlaneOutput: true,
	}

	var err error

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = eascheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping clients")

	kubeClient = kubernetes.NewForConfigOrDie(cfg)
	eaClient = eaclientset.NewForConfigOrDie(cfg)
	openfaasClient = openfaasclientsent.NewForConfigOrDie(cfg)

	By("bootstrapping informers")

	eaInformerFactory := eainformers.NewSharedInformerFactory(eaClient, time.Second*30)
	coreInformerFactory := coreinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	openfaasInformerFactory := openfaasinformers.NewSharedInformerFactory(openfaasClient, time.Minute*30)

	By("creating informers")
	informers := informers.Informers{

		Pod:                    coreInformerFactory.Core().V1().Pods(),
		Node:                   coreInformerFactory.Core().V1().Nodes(),
		Service:                coreInformerFactory.Core().V1().Services(),
		Deployment:             coreInformerFactory.Apps().V1().Deployments(),
		CommunitySchedule:      eaInformerFactory.Edgeautoscaler().V1alpha1().CommunitySchedules(),
		CommunityConfiguration: eaInformerFactory.Edgeautoscaler().V1alpha1().CommunityConfigurations(),
		Function:               openfaasInformerFactory.Openfaas().V1().Functions(),
	}

	communityController = comcontroller.NewController(
		kubeClient,
		eaClient,
		informers,
		communityNamespace,
		communityName,
	)

	By("starting informers")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	Expect(err).ToNot(HaveOccurred())

	eaInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)
	openfaasInformerFactory.Start(stopCh)

	By("starting controller")

	go func() {
		err = communityController.Run(2, stopCh)
		Expect(err).ToNot(HaveOccurred())
		close(done)
	}()

	setup()

	Eventually(done, timeout).Should(BeClosed())
}, 15)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	tearDown()
	communityController.Shutdown()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func setup() {

	newFakeSchedulerServer()
	ctx := context.TODO()

	// Create community configuration
	_, err := eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(cc.Namespace).Create(ctx, cc, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("failed to create community configuration %s/%s with error %s", cc.Namespace, cc.Name, err)
	}

	// Create the corresponding community schedules
	for _, community := range communities {
		communitySchedule := cs.DeepCopy()
		communitySchedule.Name = community
		_, err = eaClient.EdgeautoscalerV1alpha1().CommunitySchedules(communitySchedule.Namespace).Create(ctx, communitySchedule, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("failed to create community schedule %s/%s with error %s", communitySchedule.Namespace, communitySchedule.Name, err)
		}
	}

	// Create the openfaas function
	_, err = openfaasClient.OpenfaasV1().Functions(function.Namespace).Create(ctx, function, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("failed to create function %s/%s with error %s", function.Namespace, function.Name, err)
	}

	// Retrieve the nodes and label them
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("failed to list nodes with error %s", err)
	}
	for i, node := range nodes.Items {
		_, isMaster := node.Labels[ealabels.MasterNodeLabel]
		if !isMaster {
			if node.Labels == nil {
				node.Labels = make(map[string]string)
			}
			node.Labels[ealabels.CommunityLabel.WithNamespace(namespace).String()] = cc.Status.Communities[i%len(cc.Status.Communities)]
			_, err = kubeClient.CoreV1().Nodes().Update(ctx, &node, metav1.UpdateOptions{})
			if err != nil {
				klog.Errorf("failed to update node %s with error %s", node.Name, err)
			}
			workerNodes = append(workerNodes, node.DeepCopy())
		}

	}
}

func tearDown() {

	ctx := context.TODO()

	// Create community configuration
	err := eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(cc.Namespace).Delete(ctx, cc.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("failed to delete community configuration %s/%s with error %s", cc.Namespace, cc.Name, err)
	}

	// Create the corresponding community schedules
	for _, community := range communities {
		communitySchedule := cs.DeepCopy()
		communitySchedule.Name = community
		err = eaClient.EdgeautoscalerV1alpha1().CommunitySchedules(communitySchedule.Namespace).Delete(ctx, communitySchedule.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("failed to delete community schedule %s/%s with error %s", communitySchedule.Namespace, communitySchedule.Name, err)
		}
	}

	// Create the openfaas function
	err = openfaasClient.OpenfaasV1().Functions(function.Namespace).Delete(ctx, function.Name, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("failed to delete function %s/%s with error %s", function.Namespace, function.Name, err)
	}

	// Retrieve the nodes and label them
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("failed to list nodes with error %s", err)
	}
	for _, node := range nodes.Items {
		_, isMaster := node.Labels[ealabels.MasterNodeLabel]
		if !isMaster {
			if node.Labels == nil {
				node.Labels = make(map[string]string)
			}
			delete(node.Labels, ealabels.CommunityLabel.WithNamespace(namespace).String())
			_, err = kubeClient.CoreV1().Nodes().Update(ctx, &node, metav1.UpdateOptions{})
			if err != nil {
				klog.Errorf("failed to update node %s with error %s", node.Name, err)
			}
		}

	}
}
