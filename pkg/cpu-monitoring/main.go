package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/cpu-monitoring/pkg/scraper"

	"github.com/lterrac/edge-autoscaler/pkg/signals"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
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

	kubernetesClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	coreInformerFactory := informers.NewSharedInformerFactory(kubernetesClient, time.Second*30)

	node = getenv("NODE_NAME", "")

	podsCache := coreInformerFactory.Core().V1().Pods()

	node, err := kubernetesClient.CoreV1().Nodes().Get(context.TODO(), node, metav1.GetOptions{})

	if err != nil {
		klog.Fatalf("Error getting node: %s", err.Error())
	}

	mc := metrics.NewForConfigOrDie(cfg)

	cpuScraper, err := scraper.New(podsCache.Lister().Pods, mc.MetricsV1beta1(), node)

	if err != nil {
		klog.Fatalf("Error creating CPU scraper: %s", err.Error())
	}

	klog.Info("starting informer factory")

	coreInformerFactory.Start(stopCh)

	if err = cpuScraper.Start(stopCh, podsCache.Informer().HasSynced); err != nil {
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

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}
