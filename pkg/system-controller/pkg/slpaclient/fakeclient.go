package slpaclient

import (
	"fmt"
	"math"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/labels"
)

// FakeClient mocks the request and responses sent to SLPA
type FakeClient struct {
	Host string
}

// NewFakeClient returns a new FakeClient
func NewFakeClient() *FakeClient {
	return &FakeClient{}
}

// Communities returns a fake set of communities
func (fc *FakeClient) Communities(req *RequestSLPA) (result []Community, err error) {
	err = nil
	hostSize := float64(len(req.Hosts))
	communities := int(math.Ceil(hostSize / float64(req.Parameters.CommunitySize)))

	for i := 0; i < communities; i++ {
		result = append(result, Community{
			Name:    fmt.Sprintf("community-%d", i),
			Members: []Host{},
		})
	}

	communityIndex := 0
	for _, node := range req.Hosts {
		node.Labels[ealabels.CommunityLabel] = result[communityIndex].Name
		node.Labels[ealabels.CommunityRoleLabel.String()] = ealabels.Member.String()
		result[communityIndex].Members = append(result[communityIndex].Members, node)

		if communityIndex < communities-1 {
			communityIndex++
		} else {
			communityIndex = 0
		}
	}

	for _, community := range result {
		community.Members[0].Labels[ealabels.CommunityRoleLabel.String()] = ealabels.Leader.String()
	}

	return result, err
}

// SetHost sets the host
func (fc *FakeClient) SetHost(host string) {
	fc.Host = host
}