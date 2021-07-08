package labels

type CommunityRole string

const (
	Leader CommunityRole = "LEADER"
	Member CommunityRole = "MEMBER"
	// CommunityRoleLabel identifies a role of a node inside a community
	CommunityRoleLabel CommunityRole = "edgeautoscaler.polimi.it/role"

	// CommunityLabel defines the community a node belongs to
	CommunityLabel string = "edgeautoscaler.polimi.it/community"

	// MasterNodeLabel is the label used by Kubernetes to specify the node running the control plane
	MasterNodeLabel string = "node-role.kubernetes.io/master"
)

func (c CommunityRole) String() string {
	return string(c)
}
