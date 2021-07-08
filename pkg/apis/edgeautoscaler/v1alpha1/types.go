package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CommunityConfigurationList is a list of CommunityConfiguration
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// CommunityConfigurationList is a list of CommunityConfigurations resources
type CommunityConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CommunityConfiguration `json:"items"`
}

// CommunityConfiguration is a configuration for the autoscaling system.
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
type CommunityConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec CommunityConfigurationSpec `json:"spec"`
}

// CommunityConfigurationSpec is the spec for a CommunityConfiguration CRD
type CommunityConfigurationSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=slpa-rest
	// SlpaService is the service:port offering the SLPA algorithm
	SlpaService string `json:"slpa-service,omitempty"`
	// +kubebuilder:validation:Required
	// CommunitySize is the upper bound on the community size
	CommunitySize int64 `json:"community-size,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// MaximumDelay is the upper bound for the delay inside a community measured in milliseconds
	MaximumDelay int32 `json:"maximum-delay,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=30
	// ProbabilityThreshold is the lower bound to accept labels in SLPA algorithm. It is a percentage
	ProbabilityThreshold int32 `json:"probability-threshold,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=20
	// Iterations specifies how many times the speaking process occurs in SLPA
	Iterations int64 `json:"iterations,omitempty"`
}

// CommunityScheduleList is a list of CommunitySchedules
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CommunityScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CommunityConfiguration `json:"items"`
}

// CommunitySchedule wraps the rules to route traffic inside a community
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CommunitySchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec CommunityScheduleSpec `json:"spec"`
}

// CommunityScheduleSpec is the spec for a CommunitySchedule
type CommunityScheduleSpec struct {
	RoutingRules CommunityRoutingRules `json:"routing_rules"`
	Allocations CommunityAllocations `json:"allocations"`
}

type CommunityRoutingRules map[string]map[string]map[string]float64
type CommunityAllocations map[string]map[string]bool
