package e2e_test

import (
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

	By("bootstrapping informers")

	crdInformerFactory := eainformers.NewSharedInformerFactory(eaClient, time.Second*30)
	coreInformerFactory := coreinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	By("creating informers")
	informers := informers.Informers{
		Pod:                    coreInformerFactory.Core().V1().Pods(),
		Node:                   coreInformerFactory.Core().V1().Nodes(),
		Service:                coreInformerFactory.Core().V1().Services(),
		CommunityConfiguration: crdInformerFactory.Edgeautoscaler().V1alpha1().CommunityConfigurations(),
		CommunitySchedule:      crdInformerFactory.Edgeautoscaler().V1alpha1().CommunitySchedules(),
	}

	By("bootstrapping the community getter and updater")
	communityUpdater := syscontroller.NewCommunityUpdater(kubeClient.CoreV1().Nodes().Update, informers.GetListers().NodeLister.List)

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

	crdInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)

	By("starting controller")

	go func() {
		err = systemController.Run(1, stopCh)
		Expect(err).ToNot(HaveOccurred())
		close(done)
	}()

	Eventually(done, timeout).Should(BeClosed())
}, 15)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	systemController.Shutdown()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
