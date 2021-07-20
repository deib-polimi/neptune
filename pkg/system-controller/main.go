package main

import (
	"flag"
	openfaasclientsent "github.com/openfaas/faas-netes/pkg/client/clientset/versioned"
	openfaasinformers "github.com/openfaas/faas-netes/pkg/client/informers/externalversions"
	"time"

	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eainformers "github.com/lterrac/edge-autoscaler/pkg/generated/informers/externalversions"
	informers2 "github.com/lterrac/edge-autoscaler/pkg/informers"
	"github.com/lterrac/edge-autoscaler/pkg/signals"
	syscontroller "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/controller"
	"github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
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

	openfaasClient, err := openfaasclientsent.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	eaInformerFactory := eainformers.NewSharedInformerFactory(eaclient, time.Minute*30)
	coreInformerFactory := informers.NewSharedInformerFactory(kubernetesClient, time.Minute*30)
	openfaasInformerFactory := openfaasinformers.NewSharedInformerFactory(openfaasClient, time.Minute*30)

	// TODO: check name of this variable
	informers := informers2.Informers{
		Pod:                    coreInformerFactory.Core().V1().Pods(),
		Node:                   coreInformerFactory.Core().V1().Nodes(),
		Service:                coreInformerFactory.Core().V1().Services(),
		Deployment:             coreInformerFactory.Apps().V1().Deployments(),
		CommunitySchedule:      eaInformerFactory.Edgeautoscaler().V1alpha1().CommunitySchedules(),
		CommunityConfiguration: eaInformerFactory.Edgeautoscaler().V1alpha1().CommunityConfigurations(),
		Function:               openfaasInformerFactory.Openfaas().V1().Functions(),
	}

	communityUpdater := syscontroller.NewCommunityUpdater(kubernetesClient.CoreV1().Nodes().Update, informers.GetListers().NodeLister.List, eaclient)

	communityGetter := slpaclient.NewClient()

	systemController := syscontroller.NewController(
		kubernetesClient,
		eaclient,
		informers,
		communityUpdater,
		communityGetter,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered sainformers in a dedicated goroutine.
	eaInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)
	openfaasInformerFactory.Start(stopCh)

	if err = systemController.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running system controller: %s", err.Error())
	}
	defer systemController.Shutdown()

	<-stopCh
	klog.Info("Shutting down workers")

}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
