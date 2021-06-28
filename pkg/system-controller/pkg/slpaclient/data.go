package slpaclient

import (
	eaapi "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// RequestSLPA is used as input by the SLPA algorithm
type RequestSLPA struct {
	Parameters  eaapi.CommunitySettingsSpec `json:"parameters"`
	Hosts       []Host                      `json:"hosts"`
	DelayMatrix DelayMatrix                 `json:"delay-matrix`
}

// Host keeps track of a node and its labels
type Host struct {
	Name   string                 `json:"name"`
	Labels map[string]interface{} `json:"labels"`
}

// DelayMatrix contains the delays between each pair of nodes
type DelayMatrix struct {
	Delays [][]float32 `json:"routes"`
}

// ResponseSLPA wraps the communities generated by SLPA
type ResponseSLPA struct {
	Communities []Community `json:"communities"`
}

// Community contains the community leader and its members
type Community struct {
	Name    string `json:"name"`
	Members []Host `json:"members"`
}

// NewRequestSLPA fills the JSON used by the SLPA algorithm
func NewRequestSLPA(cs *eaapi.CommunitySettings, nodes []*corev1.Node, delays [][]float32) *RequestSLPA {
	var hosts []Host

	for _, node := range nodes {
		hosts = append(hosts, Host{
			Name:   node.Name,
			Labels: make(map[string]interface{}),
		})
	}

	return &RequestSLPA{
		Parameters:  cs.DeepCopy().Spec,
		Hosts:       hosts,
		DelayMatrix: DelayMatrix{delays},
	}

}
