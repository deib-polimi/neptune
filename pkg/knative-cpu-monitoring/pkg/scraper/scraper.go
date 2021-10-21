package scraper

import (
	"context"
	"fmt"
	"strings"
	"time"

	cc "github.com/lterrac/edge-autoscaler/pkg/community-controller/pkg/controller"
	"github.com/lterrac/edge-autoscaler/pkg/cpu-monitoring/pkg/persistor"
	"github.com/lterrac/edge-autoscaler/pkg/db"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	"github.com/lterrac/edge-autoscaler/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	tcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

type Scraper interface {
	// start scraping CPU usage of pods running on the node
	Start(stopCh <-chan struct{}) error
	Stop()
}

type scraper struct {
	pods         func(namespace string) tcorev1.PodInterface
	metrics      v1beta1.MetricsV1beta1Interface
	persistor    persistor.Persistor
	resourceChan chan metrics.RawResourceData
	node         string
}

func New(pods func(namespace string) tcorev1.PodInterface, metricsClient v1beta1.MetricsV1beta1Interface, node *corev1.Node) (Scraper, error) {
	resourceChan := make(chan metrics.RawResourceData, 100)
	persistor := persistor.NewResourcePersistor(db.NewDBOptions(), resourceChan)

	err := persistor.SetupDBConnection()

	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %v", err)
	}

	return &scraper{
		pods:         pods,
		metrics:      metricsClient,
		persistor:    persistor,
		resourceChan: resourceChan,
		node:         node.Name,
	}, nil
}

func (s *scraper) Start(stopCh <-chan struct{}) error {
	go s.persistor.Persist()

	klog.Info("start cpu scraping")

	// TODO tune the frequency
	go wait.Until(s.scrape, 5*time.Second, stopCh)

	return nil
}

func (s *scraper) Stop() {
	s.persistor.Stop()
}

func (s *scraper) scrape() {
	pods, err := s.pods(corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{
			"spec.nodeName": s.node,
		}).String(),
	})

	klog.Infof("scraping %d pods", len(pods.Items))

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to retrieve pods in node %v: %v", s.node, err))
		return
	}

	var total int64

	for _, p := range pods.Items {

		total = 0

		m, err := s.metrics.PodMetricses(p.Namespace).Get(context.TODO(), p.Name, metav1.GetOptions{})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to retrieve metrics for pod %v: %v", p.Name, err))
			continue
		}

		klog.Infof("scraping pod %v", p.Name)

		for _, c := range m.Containers {
			//Http metrics proxy should not be considere in CPU usage
			if strings.Contains(c.Name, cc.HttpMetrics) {
				continue
			}

			klog.Infof("container %v usage cpu %v", c.Name, c.Usage.Cpu().MilliValue())
			total += c.Usage.Cpu().MilliValue()
		}

		namespace := p.Labels[ealabels.FunctionNamespaceLabel]
		// save to metrics database
		s.resourceChan <- metrics.RawResourceData{
			Timestamp: time.Now(),
			Node:      s.node,
			Function:  p.Labels[ealabels.FunctionNameLabel],
			Namespace: namespace,
			Community: p.Labels[ealabels.CommunityLabel.WithNamespace(namespace).String()],
			Cores:     total,
		}
	}
}
