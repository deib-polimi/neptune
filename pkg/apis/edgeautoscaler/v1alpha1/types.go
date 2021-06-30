package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// CommunityConfigurationList is a list of CommunitySettings resources
type CommunityConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CommunityConfiguration `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// CommunityConfiguration is a configuration for the autoscaling system.
type CommunityConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec CommunityConfigurationSpec `json:"spec"`
}

type CommunityConfigurationSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=slpa-rest
	SlpaService string `json:"slpa-service,omitempty"`
	// +kubebuilder:validation:Required
	CommunitySize int64 `json:"community-size,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	MaximumDelay int32 `json:"maximum-delay,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=30
	ProbabilityThreshold int32 `json:"probability-threshold,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=20
	Iterations int64 `json:"iterations,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CommunityScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CommunityConfiguration `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CommunitySchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec CommunityScheduleSpec `json:"spec"`
}

type CommunityScheduleSpec struct {
}
