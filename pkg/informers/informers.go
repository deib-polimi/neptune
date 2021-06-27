package informers

import (
	sainformers "github.com/lterrac/edge-autoscaler/pkg/generated/informers/externalversions/edgeautoscaler/v1alpha1"
	salisters "github.com/lterrac/edge-autoscaler/pkg/generated/listers/edgeautoscaler/v1alpha1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
)

type Informers struct {
	Pod                 coreinformers.PodInformer
	Node                coreinformers.NodeInformer
	Service             coreinformers.ServiceInformer
	CommunitySettingses sainformers.CommunitySettingsInformer
	CommunitySchedule   sainformers.CommunityScheduleInformer
}

func (i *Informers) GetListers() Listers {
	return Listers{
		i.Pod.Lister(),
		i.Node.Lister(),
		i.Service.Lister(),
		i.CommunitySettingses.Lister(),
		i.CommunitySchedule.Lister(),
	}
}

type Listers struct {
	corelisters.PodLister
	corelisters.NodeLister
	corelisters.ServiceLister
	salisters.CommunitySettingsLister
	salisters.CommunityScheduleLister
}
