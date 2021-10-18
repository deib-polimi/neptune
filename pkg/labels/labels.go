package labels

import (
	"fmt"
	"strings"
)

type CommunityRole string
type Community string
type CommunityInstances string

const (
	// CommunityLabel defines the community a node belongs to
	CommunityLabel Community = "edgeautoscaler.polimi.it/community"

	// MasterNodeLabel is the label used by Kubernetes to specify the node running the control plane
	MasterNodeLabel string = "node-role.kubernetes.io/master"

	// CommunityControllerDeployment is the label used to specify that a deployment is a community controller
	CommunityControllerDeploymentLabel string = "edgeautoscaler.polimi.it/community-controller"

	// CommunityInstances is the label used to identify the number replicas a community
	// desires for a certain deployment
	CommunityInstancesLabel CommunityInstances = "edgeautoscaler.polimi.it/{namespace}.{name}.instances"

	// FunctionNamespaceKey is the label used to identify the function namespace related to the resource
	FunctionNamespaceLabel string = "edgeautoscaler.polimi.it/function-namespace"

	// FunctionNameKey is the label used to identify the function name related to the resource
	FunctionNameLabel string = "edgeautoscaler.polimi.it/function-name"

	// FunctionKey is the label used to identify the node related to the resource
	NodeLabel string = "edgeautoscaler.polimi.it/node"

	// Nodes labels
	GpuLabel       string = "edgeautoscaler.polimi.it/gpu"
	GpuMemoryLabel string = "edgeautoscaler.polimi.it/gpu-memory"

	// Functions labels
	GpuFunctionLabel       string = "edgeautoscaler.polimi.it/gpu-function"
	GpuFunctionMemoryLabel string = "edgeautoscaler.polimi.it/gpu-function-memory"
	FunctionMaxDelayLabel  string = "edgeautoscaler.polimi.it/function-max-delay"

	// WorkerLabel is used to mark nodes as workers
	WorkerLabel      string = "edgeautoscaler.polimi.it/node"
	WorkerLabelValue string = "worker"
)

func (c Community) WithNamespace(ns string) Community {
	parts := strings.Split(c.String(), "/")
	prefix := parts[0]
	suffix := strings.Split(parts[1], ".")[0]
	return Community(fmt.Sprintf("%s/%s.%s", prefix, ns, suffix))
}

func (c Community) String() string {
	return string(c)
}

func (c CommunityRole) WithNamespace(ns string) CommunityRole {
	parts := strings.Split(c.String(), "/")
	prefix := parts[0]
	suffix := strings.Split(parts[1], ".")[0]
	return CommunityRole(fmt.Sprintf("%s/%s.%s", prefix, ns, suffix))
}

func (c CommunityRole) String() string {
	return string(c)
}

func (c CommunityInstances) WithNamespace(namespace string) CommunityInstances {
	parts := strings.Split(c.String(), "/")
	prefix := parts[0]
	suffix := strings.Split(parts[1], ".")
	nameSuffix := suffix[1]
	resourceSuffix := suffix[2]
	return CommunityInstances(fmt.Sprintf("%s/%s.%s.%s", prefix, namespace, nameSuffix, resourceSuffix))
}

func (c CommunityInstances) WithName(name string) CommunityInstances {
	parts := strings.Split(c.String(), "/")
	prefix := parts[0]
	suffix := strings.Split(parts[1], ".")
	namespaceSuffix := suffix[0]
	resourceSuffix := suffix[2]
	return CommunityInstances(fmt.Sprintf("%s/%s.%s.%s", prefix, namespaceSuffix, name, resourceSuffix))
}

func (c CommunityInstances) String() string {
	return string(c)
}
