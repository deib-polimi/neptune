package main

import (
	webhook "github.com/lterrac/edge-autoscaler/pkg/function-deployment-webhook/pkg/controller"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {

	schedulerName := getEnv("SCHEDULER_NAME", "edge-scheduler")
	namespaces := getEnv("NAMESPACES", "wow,openfaas-fn")
	certPath := getEnv("CERT_PATH", "./server-cert.pem")
	keyPath := getEnv("KEYPATH", "server-key.pem")

	namespacesList := strings.Split(namespaces, ",")

	config := webhook.NewConfig(schedulerName, certPath, keyPath, namespacesList)
	server := webhook.NewServer(config)

	klog.Infof("Starting webhook server")
	server.Start()
	klog.Infof("Webhook server started")

	// listening OS shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	klog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	server.Shutdown()

}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}
