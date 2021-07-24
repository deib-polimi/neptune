package controller

import (
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
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

// Deployment events

func (c *SystemController) handleDeploymentAdd(new interface{}) {
	dp, ok := new.(*appsv1.Deployment)
	if !ok {
		klog.Infof("new object %s is not a community configuration", new)
		return
	}
	if isFunctionDeployment(dp) {
		c.syncDeploymentReplicasWorkqueue.Enqueue(new)
	}
}

func (c *SystemController) handleDeploymentDelete(old interface{}) {
	dp, ok := old.(*appsv1.Deployment)
	if !ok {
		klog.Infof("old object %s is not a community configuration", old)
		return
	}
	if isFunctionDeployment(dp) {
		c.syncDeploymentReplicasWorkqueue.Enqueue(old)
	}
}

func (c *SystemController) handleDeploymentUpdate(old, new interface{}) {
	dp, ok := new.(*appsv1.Deployment)
	if !ok {
		klog.Infof("new object %s is not a community configuration", new)
		return
	}
	if isFunctionDeployment(dp) {
		c.syncDeploymentReplicasWorkqueue.Enqueue(new)
	}
}

// isFunctionDeployment returns true if the deployment has a corresponding function
func isFunctionDeployment(dp *appsv1.Deployment) bool {
	if dp.ObjectMeta.Annotations == nil {
		return false
	}
	_, ok := dp.ObjectMeta.Annotations["com.openfaas.function.spec"]
	if !ok {
		return false
	}
	return true
}
