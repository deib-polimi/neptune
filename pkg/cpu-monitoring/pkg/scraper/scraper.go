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
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

type Scraper interface {
	// start scraping CPU usage of pods running on the node
	Start(stopCh <-chan struct{}, hasSynced cache.InformerSynced) error
	Stop()
}

// by default the scraping strategy polls every pod
// running in the node using the fieldSelector
type defaultScraper struct {
	pods         func(selector labels.Selector) ([]*corev1.Pod, error)
	metrics      v1beta1.MetricsV1beta1Interface
	persistor    persistor.Persistor
	resourceChan chan metrics.RawResourceData
}

// DefaultScraper scrapes all pods running on the node
func DefaultScraper(pods func(selector labels.Selector) ([]*corev1.Pod, error), metricsClient v1beta1.MetricsV1beta1Interface) (Scraper, error) {
	resourceChan := make(chan metrics.RawResourceData, 1000)
	persistor := persistor.NewResourcePersistor(db.NewDBOptions(), resourceChan)

	err := persistor.SetupDBConnection()

	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %v", err)
	}

	return &defaultScraper{
		pods:         pods,
		metrics:      metricsClient,
		persistor:    persistor,
		resourceChan: resourceChan,
	}, nil
}

func (s *defaultScraper) Start(stopCh <-chan struct{}, hasSynced cache.InformerSynced) error {
	klog.Info("wait for cache to sync")

	if ok := cache.WaitForCacheSync(
		stopCh,
		hasSynced,
	); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	go s.persistor.Persist()

	klog.Info("start cpu scraping")

	go wait.Until(s.scrape, 5*time.Second, stopCh)

	return nil
}

func (s *defaultScraper) Stop() {
	s.persistor.Stop()
}

func (s *defaultScraper) scrape() {
	pods, err := s.pods(labels.Everything())

	klog.Infof("scraping %d pods", len(pods))

	if err != nil {
		klog.Errorf("failed to list pods: %v", err)
		return
	}

	var totalCores int64
	var totalRequests int64
	var totalLimits int64

	for _, p := range pods {

		totalCores = 0
		totalRequests = 0
		totalLimits = 0

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

			for _, pc := range p.Spec.Containers {
				if pc.Name != c.Name {
					continue
				}

				r, rExists := pc.Resources.Requests[corev1.ResourceCPU]
				l, lExists := pc.Resources.Limits[corev1.ResourceCPU]

				if rExists && lExists {
					totalRequests += r.MilliValue()
					totalLimits += l.MilliValue()
				}
			}

			klog.Infof("container %v usage cpu %v", c.Name, c.Usage.Cpu().MilliValue())
			totalCores += c.Usage.Cpu().MilliValue()
		}

		namespace := p.Labels[ealabels.FunctionNamespaceLabel]
		// save to metrics database
		s.resourceChan <- metrics.RawResourceData{
			Timestamp: time.Now(),
			Node:      p.Spec.NodeName,
			Function:  p.Labels[ealabels.FunctionNameLabel],
			Namespace: namespace,
			Community: p.Labels[ealabels.CommunityLabel.WithNamespace(namespace).String()],
			Cores:     totalCores,
			Requests:  totalRequests,
			Limits:    totalLimits,
		}
	}
}
