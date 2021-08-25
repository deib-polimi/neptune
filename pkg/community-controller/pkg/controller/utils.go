package controller

import (
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCommunitySchedule returns a new empty community schedule with a given namespace and name
func NewCommunitySchedule(namespace, name string) *eav1alpha1.CommunitySchedule {
	return &eav1alpha1.CommunitySchedule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "edgeautoscaler.polimi.it/v1alpha1",
			Kind:       "CommunitySchedule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: name},
		Spec: eav1alpha1.CommunityScheduleSpec{
			RoutingRules: make(eav1alpha1.CommunitySourceRoutingRule),
			Allocations:  make(eav1alpha1.CommunityFunctionAllocation),
		},
	}
}
