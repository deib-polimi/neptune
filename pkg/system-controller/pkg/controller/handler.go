package controller

import (
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	"k8s.io/klog/v2"
	"reflect"
)

// Community Configuration events

func (c *SystemController) handleCommunityConfigurationsAdd(new interface{}) {
	c.syncConfigurationsWorkqueue.Enqueue(new)
	c.syncSchedulesWorkqueue.Enqueue(new)
}

func (c *SystemController) handleCommunityConfigurationsDeletion(old interface{}) {
	c.syncConfigurationsWorkqueue.Enqueue(old)
	c.syncSchedulesWorkqueue.Enqueue(old)
}

func (c *SystemController) handleCommunityConfigurationsUpdate(old, new interface{}) {
	oldcc, ok := old.(*eav1alpha1.CommunityConfiguration)
	if !ok {
		klog.Info("old %s is not a community configuration")
		return
	}
	newcc, ok := new.(*eav1alpha1.CommunityConfiguration)
	if !ok {
		klog.Info("new %s is not a community configuration")
		return
	}

	if !reflect.DeepEqual(oldcc.Spec, newcc.Spec) {
		// If spec changed, recompute the communities
		c.syncConfigurationsWorkqueue.Enqueue(new)
	}
	//if !reflect.DeepEqual(oldcc.Status, newcc.Status) {
		// If status changed, resync the community schedules
	c.syncSchedulesWorkqueue.Enqueue(new)
	//}
}

// Deployment events

func (c *SystemController) handleDeploymentAdd(new interface{}) {
	c.syncDeploymentReplicasWorkqueue.Enqueue(new)
}

func (c *SystemController) handleDeploymentDelete(old interface{}) {
	c.syncDeploymentReplicasWorkqueue.Enqueue(old)
}

func (c *SystemController) handleDeploymentUpdate(old, new interface{}) {
	c.syncDeploymentReplicasWorkqueue.Enqueue(new)
}
