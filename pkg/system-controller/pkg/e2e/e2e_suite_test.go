package e2e_test

import (
	"context"
	openfaasclientsent "github.com/openfaas/faas-netes/pkg/client/clientset/versioned"
	openfaasinformers "github.com/openfaas/faas-netes/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"testing"
	"time"

	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eascheme "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned/scheme"
	eainformers "github.com/lterrac/edge-autoscaler/pkg/generated/informers/externalversions"
	"github.com/lterrac/edge-autoscaler/pkg/informers"
	"github.com/lterrac/edge-autoscaler/pkg/signals"
	syscontroller "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/controller"
	"github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
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
var systemController *syscontroller.SystemController

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

	By("bootstrapping the community getter and updater")
	communityUpdater := syscontroller.NewCommunityUpdater(kubeClient.CoreV1().Nodes().Update, informers.GetListers().NodeLister.List, eaClient)

	communityGetter := slpaclient.NewFakeClient()
	By("bootstrapping controller")

	systemController = syscontroller.NewController(
		kubeClient,
		eaClient,
		informers,
		communityUpdater,
		communityGetter,
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
		err = systemController.Run(1, stopCh)
		Expect(err).ToNot(HaveOccurred())
		close(done)
	}()

	setup()

	Eventually(done, timeout).Should(BeClosed())
}, 15)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	systemController.Shutdown()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})


func setup() {
	nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	Expect(err).ShouldNot(HaveOccurred())

	// wait for nodes to become ready
	Eventually(func() bool {
		nodes, err = kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ShouldNot(HaveOccurred())

		for _, node := range nodes.Items {
			for _, condition := range node.Status.Conditions {
				klog.Info("Node name: %s, Conditions: %s-%s", condition.Type, condition.Status)
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionFalse {
					return false
				}
			}
		}
		return true

	}, 15*timeout, interval).Should(BeTrue())

	_, err = eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(namespace).Create(context.TODO(), cc, metav1.CreateOptions{})
	Expect(err).ShouldNot(HaveOccurred())
}
