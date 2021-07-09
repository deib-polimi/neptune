package labels

import (
	"fmt"
	"strings"
)

type CommunityRole string
type Community string

const (
	Leader CommunityRole = "LEADER"
	Member CommunityRole = "MEMBER"
	// CommunityRoleLabel identifies a role of a node inside a community
	CommunityRoleLabel CommunityRole = "edgeautoscaler.polimi.it.role"

	// CommunityLabel defines the community a node belongs to
	CommunityLabel Community = "edgeautoscaler.polimi.it.community"

	// MasterNodeLabel is the label used by Kubernetes to specify the node running the control plane
	MasterNodeLabel string = "node-role.kubernetes.io/master"
)

func (c Community) WithNamespace(ns string) Community {
	prefix := strings.Split(c.String(), "/")[0]
	return Community(fmt.Sprintf("%s/%s", prefix, ns))
}

func (c Community) String() string {
	return string(c)
}

func (c CommunityRole) WithNamespace(ns string) CommunityRole {
	prefix := strings.Split(c.String(), "/")[0]
	return CommunityRole(fmt.Sprintf("%s/%s", prefix, ns))
}

func (c CommunityRole) String() string {
	return string(c)
}
