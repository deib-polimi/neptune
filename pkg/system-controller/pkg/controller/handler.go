package controller

import (
	"reflect"

	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	"k8s.io/klog/v2"
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
		klog.Infof("old object %s is not a community configuration", old)
		return
	}
	newcc, ok := new.(*eav1alpha1.CommunityConfiguration)
	if !ok {
		klog.Infof("new object %s is not a community configuration", new)
		return
	}

	if !reflect.DeepEqual(oldcc.Spec, newcc.Spec) {
		// If spec changed, recompute the communities
		c.syncConfigurationsWorkqueue.Enqueue(new)
	}
	c.syncSchedulesWorkqueue.Enqueue(new)
}
