package controller

import (
	"fmt"
	"strings"
)

type CommunityInstances string

const (
	CommunityInstancesLabel CommunityInstances = "edgeautoscaler.polimi.it/{namespace}.{name}.instances"
)

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
