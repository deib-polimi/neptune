package informers

import (
	sainformers "github.com/lterrac/edge-autoscaler/pkg/generated/informers/externalversions/edgeautoscaler/v1alpha1"
	salisters "github.com/lterrac/edge-autoscaler/pkg/generated/listers/edgeautoscaler/v1alpha1"
	openfaasinformers "github.com/openfaas/faas-netes/pkg/client/informers/externalversions/openfaas/v1"
	openfaaslisters "github.com/openfaas/faas-netes/pkg/client/listers/openfaas/v1"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
)

type Informers struct {
	ConfigMap              coreinformers.ConfigMapInformer
	Pod                    coreinformers.PodInformer
	Node                   coreinformers.NodeInformer
	Service                coreinformers.ServiceInformer
	Deployment             appsinformers.DeploymentInformer
	CommunityConfiguration sainformers.CommunityConfigurationInformer
	CommunitySchedule      sainformers.CommunityScheduleInformer
	Function               openfaasinformers.FunctionInformer
}

func (i *Informers) GetListers() Listers {
	return Listers{
		i.ConfigMap.Lister(),
		i.Pod.Lister(),
		i.Node.Lister(),
		i.Service.Lister(),
		i.Deployment.Lister(),
		i.CommunityConfiguration.Lister(),
		i.CommunitySchedule.Lister(),
		i.Function.Lister(),
	}
}

type Listers struct {
	corelisters.ConfigMapLister
	corelisters.PodLister
	corelisters.NodeLister
	corelisters.ServiceLister
	appslisters.DeploymentLister
	salisters.CommunityConfigurationLister
	salisters.CommunityScheduleLister
	openfaaslisters.FunctionLister
}
