package controller

import (
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
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
	if ok && c.checkPod(pod) {
		cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
		if err != nil {
			klog.Errorf("Can not retrieve community schedule %s/%s, with error %v", c.communityNamespace, c.communityName, err)
			return
		}
		c.syncCommunityScheduleWorkqueue.Enqueue(cs)
	}
}

func (c *CommunityController) handlePodDelete(old interface{}) {
	pod, ok := old.(*corev1.Pod)
	if ok && c.checkPod(pod) {
		cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
		if err != nil {
			klog.Errorf("Can not retrieve community schedule %s/%s, with error %v", c.communityNamespace, c.communityName, err)
			return
		}
		c.syncCommunityScheduleWorkqueue.Enqueue(cs)
	}
}

func (c *CommunityController) handlePodUpdate(old, new interface{}) {
	pod, ok := new.(*corev1.Pod)
	if ok && c.checkPod(pod) {
		cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
		if err != nil {
			klog.Errorf("Can not retrieve community schedule %s/%s, with error %v", c.communityNamespace, c.communityName, err)
			return
		}
		c.syncCommunityScheduleWorkqueue.Enqueue(cs)
	}
	pod, ok = old.(*corev1.Pod)
	if ok && c.checkPod(pod) {
		cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
		if err != nil {
			klog.Errorf("Can not retrieve community schedule %s/%s, with error %v", c.communityNamespace, c.communityName, err)
			return
		}
		c.syncCommunityScheduleWorkqueue.Enqueue(cs)
	}
}

func (c *CommunityController) checkPod(pod *corev1.Pod) bool {
	community, ok := pod.Labels[ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String()]
	return ok && community == c.communityName
}
