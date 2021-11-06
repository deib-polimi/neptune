package v1alpha1

// +kubebuilder:object:generate=true

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CommunityConfigurationList is a list of CommunityConfiguration
// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// CommunityConfigurationList is a list of CommunityConfigurations resources
type CommunityConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CommunityConfiguration `json:"items"`
}

// CommunityConfiguration is a configuration for the autoscaling system.
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CommunityConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec CommunityConfigurationSpec `json:"spec"`
	// +kubebuilder:validation:Required
	Status CommunityConfigurationStatus `json:"status"`
}

// CommunityConfigurationSpec is the spec for a CommunityConfiguration CRD
type CommunityConfigurationSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=slpa-rest
	// SlpaService is the service:port offering the SLPA algorithm
	SlpaService string `json:"slpa-service"`
	// +kubebuilder:validation:Required
	// CommunitySize is the upper bound on the community size
	CommunitySize int64 `json:"community-size"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum=10000
	// +kubebuilder:validation:Minimum=0
	// MaximumDelay is the upper bound for the delay inside a community measured in milliseconds
	MaximumDelay int32 `json:"maximum-delay"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=30
	// ProbabilityThreshold is the lower bound to accept labels in SLPA algorithm. It is a percentage
	ProbabilityThreshold int32 `json:"probability-threshold"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=20
	// Iterations specifies how many times the speaking process occurs in SLPA
	Iterations int64 `json:"iterations"`
}

// CommunityConfigurationStatus represents the status of the community configurations.
type CommunityConfigurationStatus struct {
	// Communities represents the community generated by slpa for a given namespace
	Communities []string `json:"generated-communities"`
}

// CommunityScheduleList is a list of CommunitySchedules
// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CommunityScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CommunitySchedule `json:"items"`
}

// CommunitySchedule wraps the rules to route traffic inside a community
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CommunitySchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec CommunityScheduleSpec `json:"spec"`
}

// CommunityScheduleSpec is the spec for a CommunitySchedule
type CommunityScheduleSpec struct {
	CpuRoutingRules  CommunitySourceRoutingRule  `json:"cpu-routing-rules"`
	CpuAllocations   CommunityFunctionAllocation `json:"cpu-allocations"`
	GpuRoutingRules  CommunitySourceRoutingRule  `json:"gpu-routing-rules"`
	GpuAllocations   CommunityFunctionAllocation `json:"gpu-allocations"`
	AlgorithmService string                      `json:"algorithm-service"`
}

// routing rule format: source node - function name - destination node

type CommunityFunctionRoutingRule map[string]CommunityDestinationRoutingRule
type CommunityDestinationRoutingRule map[string]resource.Quantity
type CommunitySourceRoutingRule map[string]CommunityFunctionRoutingRule

type CommunityNodeAllocation map[string]bool
type CommunityFunctionAllocation map[string]CommunityNodeAllocation
