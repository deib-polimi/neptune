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
	corelisters "k8s.io/client-go/listers/core/v1"
)

type Scraper interface {
	// start scraping CPU usage of pods running on the node
	Start(node string)
	Stop()
}

type scraper struct {
	pods         apiutils.PodGetter
	cadvisor     *cadvisorclient.Client
	persistor    *persistor.ResourcePersistor
	resourceChan chan metrics.RawResourceData
}

func New(pods func(namespace string) corelisters.PodNamespaceLister) (*scraper, error) {
	podGetter, err := apiutils.NewPodGetter(pods)

	if err != nil {
		return nil, fmt.Errorf("failed to create podGetter: %v", err)
	}

	client, err := cadvisorclient.NewClient("")

	if err != nil {
		return nil, fmt.Errorf("failed to create cadvisor client: %v", err)
	}

	resourceChan := make(chan metrics.RawResourceData, 100)
	persistor := persistor.NewResourcePersistor(mp.NewDBOptions(), resourceChan)

	return &scraper{
		pods:         podGetter,
		cadvisor:     client,
		persistor:    persistor,
		resourceChan: resourceChan,
	}, nil
}

func (s *scraper) Start(node string) {
	pods, err := s.pods.GetPodsOfAllFunctionInNode(corev1.NamespaceAll, node)

	if err != nil {
		//TODO
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

			cpu, err := s.scrape(c.Name)

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
			Node:      node,
			Function:  p.Labels[ealabels.FunctionNameLabel],
			Namespace: namespace,
			Community: p.Labels[ealabels.CommunityLabel.WithNamespace(namespace).String()],
			Cores:     int(total),
		}
	}
}

func (s *scraper) Stop() {}

func (s *scraper) scrape(container string) (uint64, error) {

	// TODO: make an average over the last x seconds.
	// use start and end fields
	info := &cadvisorv1.ContainerInfoRequest{
		NumStats: 1,
	}

	cInfo, err := s.cadvisor.ContainerInfo(container, info)

	if err != nil {
		return 0, fmt.Errorf("failed to retrieve metrics for container %s: %v", container, err)
	}

	return cInfo.Stats[0].Cpu.Usage.Total, nil
}
