package controller

import (
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Community Schedule handlers
// Whenever a Community Schedule event is generated check if any pods is misplaced
func (c *CommunityController) handleCommunityScheduleAdd(new interface{}) {
	cs, ok := new.(*eav1alpha1.CommunitySchedule)
	if ok && cs.Namespace == c.communityNamespace && cs.Name == c.communityName {
		c.syncCommunityScheduleWorkqueue.Enqueue(new)
	}
}

func (c *CommunityController) handleCommunityScheduleDelete(old interface{}) {
	cs, ok := old.(*eav1alpha1.CommunitySchedule)
	if ok && cs.Namespace == c.communityNamespace && cs.Name == c.communityName {
		c.syncCommunityScheduleWorkqueue.Enqueue(old)
	}
}

func (c *CommunityController) handleCommunityScheduleUpdate(old, new interface{}) {
	cs, ok := new.(*eav1alpha1.CommunitySchedule)
	if ok && cs.Namespace == c.communityNamespace && cs.Name == c.communityName {
		c.syncCommunityScheduleWorkqueue.Enqueue(new)
	}
}

// Pod handlers
// Whenever a pod is unscheduled, schedule it
func (c *CommunityController) handlePodAdd(new interface{}) {
	pod, ok := new.(*corev1.Pod)
	if ok && len(pod.Spec.NodeName) == 0 {
		c.unscheduledPodWorkqueue.Enqueue(new)
	}
}

func (c *CommunityController) handlePodUpdate(old, new interface{}) {
	pod, ok := new.(*corev1.Pod)
	if ok && len(pod.Spec.NodeName) == 0 {
		c.unscheduledPodWorkqueue.Enqueue(new)
	}
}
