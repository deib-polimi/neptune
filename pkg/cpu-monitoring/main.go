package main

import (
	"flag"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/cpu-monitoring/pkg/scraper"

	"github.com/lterrac/edge-autoscaler/pkg/signals"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
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

	kubernetesClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	coreInformerFactory := informers.NewSharedInformerFactory(kubernetesClient, time.Second*30)

	mc := metrics.NewForConfigOrDie(cfg)

	cpuScraper, err := scraper.DefaultScraper(coreInformerFactory.Core().V1().Pods().Lister().List, mc.MetricsV1beta1())

	if err != nil {
		klog.Fatalf("Error creating CPU scraper: %s", err.Error())
	}

	klog.Info("starting informer factory")
	coreInformerFactory.Start(stopCh)

	if err = cpuScraper.Start(stopCh, coreInformerFactory.Core().V1().Pods().Informer().HasSynced); err != nil {
		klog.Fatalf("Error running cpu scraper: %s", err.Error())
	}
	defer cpuScraper.Stop()

	<-stopCh
	klog.Info("Shutting down workers")
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
