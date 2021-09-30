package scraper

import (
	"fmt"
	"strings"
	"time"

	cadvisorclient "github.com/google/cadvisor/client"
	cadvisorv1 "github.com/google/cadvisor/info/v1"
	"github.com/lterrac/edge-autoscaler/pkg/apiutils"
	cc "github.com/lterrac/edge-autoscaler/pkg/community-controller/pkg/controller"
	"github.com/lterrac/edge-autoscaler/pkg/cpu-monitoring/pkg/persistor"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	mp "github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/persistor"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type Scraper interface {
	// start scraping CPU usage of pods running on the node
	Start(node string, stopCh <-chan struct{}, hasSynced func() bool)
	Stop()
}

type scraper struct {
	pods         apiutils.PodGetter
	cadvisor     *cadvisorclient.Client
	persistor    persistor.Persistor
	resourceChan chan metrics.RawResourceData
	node         string
}

func New(pods func(namespace string) corelisters.PodNamespaceLister) (*scraper, error) {
	podGetter, err := apiutils.NewPodGetter(pods)

	if err != nil {
		return nil, fmt.Errorf("failed to create pod getter: %v", err)
	}

	client, err := cadvisorclient.NewClient("")

	if err != nil {
		return nil, fmt.Errorf("failed to create cadvisor client: %v", err)
	}

	resourceChan := make(chan metrics.RawResourceData, 100)
	persistor := persistor.NewResourcePersistor(mp.NewDBOptions(), resourceChan)

	err = persistor.SetupDBConnection()

	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %v", err)
	}

	return &scraper{
		pods:         podGetter,
		cadvisor:     client,
		persistor:    persistor,
		resourceChan: resourceChan,
	}, nil
}

func (s *scraper) Start(stopCh <-chan struct{}, node string, hasSynced func() bool) error {
	s.node = node

	if ok := cache.WaitForCacheSync(
		stopCh,
		hasSynced,
	); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	s.persistor.Persist()

	// TODO tune the frequency
	go wait.Until(s.scrape, 5*time.Second, stopCh)

	return nil
}

func (s *scraper) Stop() {
	s.persistor.Stop()
}

func (s *scraper) scrape() {
	pods, err := s.pods.GetPodsOfAllFunctionInNode(corev1.NamespaceAll, s.node)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to retrieve pods in node %v: %v", s.node, err))
		return
	}

	var total uint64

	for _, p := range pods {

		total = 0

		for _, c := range p.Spec.Containers {
			//Http metrics proxy should not be considere in CPU usage
			if strings.Contains(c.Image, cc.HttpMetricsImage) {
				continue
			}

			cpu, err := s.containerCPU(c.Name)

			if err != nil {
				//TODO
				break
			}

			total += cpu
		}

		namespace := p.Labels[ealabels.FunctionNamespaceLabel]
		// save to metrics database
		s.resourceChan <- metrics.RawResourceData{
			Timestamp: time.Now(),
			Node:      s.node,
			Function:  p.Labels[ealabels.FunctionNameLabel],
			Namespace: namespace,
			Community: p.Labels[ealabels.CommunityLabel.WithNamespace(namespace).String()],
			Cores:     int(total),
		}
	}
}

func (s scraper) containerCPU(container string) (uint64, error) {
	// TODO: make an average over the last x seconds.
	// use start and end fields
	info := &cadvisorv1.ContainerInfoRequest{
		Start: time.Now().Add(-5 * time.Second),
		End:   time.Now(),
	}

	cInfo, err := s.cadvisor.ContainerInfo(container, info)

	var avgCPU uint64

	for _, stat := range cInfo.Stats {
		avgCPU += stat.Cpu.Usage.Total
	}

	avgCPU = avgCPU / uint64(len(cInfo.Stats))

	if err != nil {
		return 0, fmt.Errorf("failed to retrieve metrics for container %s: %v", container, err)
	}

	return avgCPU, nil
}
