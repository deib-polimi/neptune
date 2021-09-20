package controller

import (
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// Community Schedule handlers
// Whenever a Community Schedule event is generated check if any pods is misplaced
func (c *CommunityController) handleCommunitySchedule(obj interface{}) {
	c.processCommunitySchedule(obj)
}

func (c *CommunityController) handleCommunityScheduleUpdate(old, new interface{}) {
	c.processCommunitySchedule(new)
}

// Node handlers
// Whenever a node leaves or join a community
func (c *CommunityController) handleNode(new interface{}) {
	c.processNode(new)
}

func (c *CommunityController) handleNodeUpdate(old, new interface{}) {
	c.processNode(old)
	c.processNode(new)
}

// Pod handlers
// Whenever pods are created or deleted, check if the community schedules allocations are still consistent
func (c *CommunityController) handlePod(new interface{}) {
	c.processPod(new)
}

func (c *CommunityController) handlePodUpdate(old, new interface{}) {
	c.processPod(old)
	c.processPod(new)
}

func (c *CommunityController) belongs(pod *corev1.Pod) bool {
	community, ok := pod.Labels[ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String()]
	return ok && community == c.communityName
}

// check if a pod belongs to the community managed by the coomunity controller and process the corresponding community schedule
func (c *CommunityController) processPod(podObject interface{}) {
	pod, ok := podObject.(*corev1.Pod)
	if ok && c.belongs(pod) {
		cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
		if err != nil {
			klog.Errorf("Can not retrieve community schedule %s/%s, with error %v", c.communityNamespace, c.communityName, err)
			return
		}
		c.syncCommunityScheduleWorkqueue.Enqueue(cs)
	}
}

// check if a node belongs to the community managed by the community controller and triggers the scheduling process to place pods on it
func (c *CommunityController) processNode(nodeObject interface{}) {
	if node, ok := nodeObject.(*corev1.Node); ok {
		if nodeCommunity, ok := node.Labels[ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String()]; ok {
			if nodeCommunity == c.communityName {
				_ = c.runScheduler("")
			}
		}
	}
}

// check if a community schedule belong to the controller and syncs it
func (c *CommunityController) processCommunitySchedule(csObject interface{}) {
	cs, ok := csObject.(*eav1alpha1.CommunitySchedule)
	if ok && cs.Namespace == c.communityNamespace && cs.Name == c.communityName {
		c.syncCommunityScheduleWorkqueue.Enqueue(csObject)
	}
}
