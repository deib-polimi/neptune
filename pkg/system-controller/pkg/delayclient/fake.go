package delayclient

import (
	"github.com/lterrac/edge-autoscaler/pkg/informers"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

type FakeDelayClient struct {
	listers informers.Listers
}

func NewFakeClient(l informers.Listers) *FakeDelayClient {
	return &FakeDelayClient{
		listers: l,
	}
}

func (f FakeDelayClient) GetDelays() ([]*NodeDelay, error) {
	nodes, err := f.listers.NodeLister.List(labels.Everything())
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	delays := make([]*NodeDelay, 0)
	for _, from := range nodes {
		for _, to := range nodes {
			delays = append(delays, &NodeDelay{
				FromNode: from.Name,
				ToNode:   to.Name,
				Latency:  0,
			})
		}
	}

	return delays, nil
}
