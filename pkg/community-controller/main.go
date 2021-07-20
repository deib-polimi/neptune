package main

import (
	"flag"
	"os"
	"time"

	comcontroller "github.com/lterrac/edge-autoscaler/pkg/community-controller/pkg/controller"
	eaclientset "github.com/lterrac/edge-autoscaler/pkg/generated/clientset/versioned"
	eainformers "github.com/lterrac/edge-autoscaler/pkg/generated/informers/externalversions"
	informers2 "github.com/lterrac/edge-autoscaler/pkg/informers"
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

	communityName := getenv("COMMUNITY_NAME", "community-worker")
	communityNamespace := getenv("COMMUNITY_NAMESPACE", "openfaas-fn")

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

	communityController := comcontroller.NewController(
		kubernetesClient,
		eaclient,
		informers,
		communityNamespace,
		communityName,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered sainformers in a dedicated goroutine.
	eaInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)
	openfaasInformerFactory.Start(stopCh)

	if err = communityController.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running system controller: %s", err.Error())
	}

	defer communityController.Shutdown()

	<-stopCh
	klog.Info("Shutting down workers")

}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}
