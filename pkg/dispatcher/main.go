package main

import (
	"flag"
	"time"

	lbcontroller "github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/controller"
	monitoringmetrics "github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eainformers "github.com/lterrac/edge-autoscaler/pkg/generated/informers/externalversions"
	informerswrapper "github.com/lterrac/edge-autoscaler/pkg/informers"
	openfaasclientsent "github.com/openfaas/faas-netes/pkg/client/clientset/versioned"
	openfaasinformers "github.com/openfaas/faas-netes/pkg/client/informers/externalversions"

	"github.com/lterrac/edge-autoscaler/pkg/signals"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	masterURL  string
	kubeconfig string
	node       string
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

	eaInformerFactory := eainformers.NewSharedInformerFactory(eaclient, time.Second*30)
	coreInformerFactory := informers.NewSharedInformerFactory(kubernetesClient, time.Second*30)
	openfaasInformerFactory := openfaasinformers.NewSharedInformerFactory(openfaasClient, time.Minute*30)

	informers := informerswrapper.Informers{
		Pod:                    coreInformerFactory.Core().V1().Pods(),
		Node:                   coreInformerFactory.Core().V1().Nodes(),
		Service:                coreInformerFactory.Core().V1().Services(),
		Deployment:             coreInformerFactory.Apps().V1().Deployments(),
		CommunitySchedule:      eaInformerFactory.Edgeautoscaler().V1alpha1().CommunitySchedules(),
		CommunityConfiguration: eaInformerFactory.Edgeautoscaler().V1alpha1().CommunityConfigurations(),
		Function:               openfaasInformerFactory.Openfaas().V1().Functions(),
	}

	monitoringChan := make(chan monitoringmetrics.RawMetricData)

	lbController := lbcontroller.NewController(
		kubernetesClient,
		eaclient,
		informers,
		monitoringChan,
		node,
	)

	coreInformerFactory.Start(stopCh)
	eaInformerFactory.Start(stopCh)
	openfaasInformerFactory.Start(stopCh)

	if err = lbController.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running system controller: %s", err.Error())
	}
	defer lbController.Shutdown()

	<-stopCh
	klog.Info("Shutting down workers")
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&node, "node", "", "The node on which the dispatcher is running")
}
