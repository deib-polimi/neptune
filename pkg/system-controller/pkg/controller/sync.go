package controller

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	fp "github.com/JohnCGriffin/yogofn"
	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	slpaclient "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/slpaclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	// EmptyNodeListError is the default error message when grouping cluster nodes
	EmptyNodeListError string = "there are no or too few ready nodes for building communities"
)

// TODO: better error handling
func (c *SystemController) syncCommunityConfiguration(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	klog.Info(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CC resource with this namespace/name
	cc, err := c.listers.CommunityConfigurationLister.CommunityConfigurations(namespace).Get(name)

	//TODO: handle multiple Community Settings in cluster. Now there should be ONLY one configuration per cluster
	if err != nil {
		// The CC resource may no longer exist, so we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("CommunityConfiguraton '%s' in work queue no longer exists", key))

			klog.Info("Clearing nodes' labels")
			c.communityUpdater.ClearNodesLabels(namespace)
			return nil
		}
		return err
	}

	communities, err := c.ComputeCommunities(cc)

	if err != nil {
		return fmt.Errorf("error while executing SLPA: %s", err)
	}

	//add the community label with the corresponding namespace
	for _, community := range communities {
		for _, member := range community.Members {
			member.Labels[ealabels.CommunityLabel.WithNamespace(cc.Namespace).String()] = community.Name
		}
	}

	// update labels on corev1.Node with the corresponding community
	err = c.communityUpdater.UpdateCommunityNodes(cc.Namespace, communities)

	if err != nil {
		return fmt.Errorf("error while updating nodes: %s", err)
	}

	newCCStatus, _ := fp.Map(CommunityName, communities).([]string)

	err = c.communityUpdater.UpdateConfigurationStatus(cc, newCCStatus)

	if err != nil {
		return fmt.Errorf("error while updating %s status: %s", key, err)
	}

	c.recorder.Event(cc, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// ComputeCommunities divides cluster nodes into communities according to the settings passed as input
// using SLPA
func (c *SystemController) ComputeCommunities(cc *eav1alpha1.CommunityConfiguration) ([]slpaclient.Community, error) {
	// create the input JSON request used by SLPA algorithm
	req, err := c.fetchSLPAData(cc)

	if err != nil {
		return nil, fmt.Errorf("fetching slpa data failed: %s", err)
	}

	// send the request to SLPA and read results
	return c.communityGetter.Communities(req)
}

func (c *SystemController) fetchSLPAData(cc *eav1alpha1.CommunityConfiguration) (*slpaclient.RequestSLPA, error) {

	// Get all nodes in cluster
	nodes, err := c.listers.NodeLister.List(labels.Everything())

	if err != nil {
		return nil, err
	}

	// Keep only the ones in ready state.
	nodes, err = filterReadyNodes(nodes)

	if err != nil {
		return nil, fmt.Errorf("error while filtering ready nodes: %s", err)
	}

	// Get delay matrix
	delays, err := c.getNodeDelays(nodes)

	if err != nil {
		return nil, fmt.Errorf("error while retrieving node delays: %s", err)
	}

	c.communityGetter.SetHost(cc.Spec.SlpaService)

	request := slpaclient.NewRequestSLPA(cc, nodes, delays)

	return request, nil
}

func filterReadyNodes(nodes []*corev1.Node) (result []*corev1.Node, err error) {
	// TODO: once delay discovery is implemented modify it according to changes

	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {

			// don't consider master nodes for building communities
			if _, isMaster := node.Labels[ealabels.MasterNodeLabel]; isMaster {
				continue
			}

			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				result = append(result, node)
			}
		}
	}

	if len(result) < 1 {
		return result, fmt.Errorf(EmptyNodeListError)
	}

	return result, nil
}

// TODO: Up to this point the delay matrix is hard coded
func (c *SystemController) getNodeDelays(nodes []*corev1.Node) (delays [][]int32, err error) {
	delays = make([][]int32, len(nodes))

	for i := range delays {
		delays[i] = make([]int32, len(nodes))
	}

	// TODO: refactor once delay discovery implemented
	err = nil
	for source := range nodes {
		for destination := range nodes {
			value := 2

			if source == destination {
				value = 0
			}
			delays[source][destination] = int32(value)
		}
	}

	return
}

func (c *SystemController) syncCommunitySchedules(key string) error {

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CC resource with this namespace/name
	cc, err := c.listers.CommunityConfigurationLister.CommunityConfigurations(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			klog.Errorf("failed to retrieve community configuration %s/%s, error: %s", namespace, name, err)
			return err
		}
	}

	// Check if there's any CommunitySchedule resource which should be deleted or created
	css, err := c.listers.CommunitySchedules(namespace).List(labels.Everything())
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			klog.Errorf("failed to list community schedules in namespace %s, error: %s", namespace, err)
			return err
		}
	}

	cssMap := make(map[string]*eav1alpha1.CommunitySchedule, len(css))
	for _, cs := range css {
		cssMap[cs.Name] = cs
	}
	for _, community := range cc.Status.Communities {
		if _, ok := cssMap[community]; !ok {
			cs := NewCommunitySchedule(namespace, community)
			_, err = c.edgeAutoscalerClientSet.EdgeautoscalerV1alpha1().CommunitySchedules(cs.Namespace).Create(context.TODO(), cs, metav1.CreateOptions{})
			if err != nil {
				klog.Info(err)
				return err
			}
		} else {
			delete(cssMap, community)
		}
	}
	for _, inconsistentCs := range cssMap {
		err = c.edgeAutoscalerClientSet.EdgeautoscalerV1alpha1().CommunitySchedules(inconsistentCs.Namespace).Delete(context.TODO(), inconsistentCs.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Info(err)
			return err
		}
	}

	selector := labels.SelectorFromSet(map[string]string{
		ealabels.CommunityControllerDeploymentLabel: "",
	})
	dps, err := c.listers.Deployments(namespace).List(selector)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			klog.Errorf("failed to list deployments in namespace, error: %s", namespace, err)
			return err
		}
	}

	dpsMap := make(map[string]*appsv1.Deployment, len(dps))
	for _, dp := range dps {
		dpsMap[dp.Name] = dp
	}
	for _, community := range cc.Status.Communities {
		if _, ok := dpsMap[community]; !ok {
			dp := NewCommunityController(namespace, community)
			_, err = c.kubernetesClientset.AppsV1().Deployments(dp.Namespace).Create(context.TODO(), dp, metav1.CreateOptions{})
			if err != nil {
				klog.Info(err)
				return err
			}
		} else {
			delete(dpsMap, community)
		}
	}
	for _, inconsistentDp := range dpsMap {
		err = c.kubernetesClientset.AppsV1().Deployments(inconsistentDp.Namespace).Delete(context.TODO(), inconsistentDp.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Info(err)
			return err
		}
	}

	return nil

}

// NewCommunitySchedule returns a new empty community schedule with a given namespace and name
func NewCommunitySchedule(namespace, name string) *eav1alpha1.CommunitySchedule {
	return &eav1alpha1.CommunitySchedule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "edgeautoscaler.polimi.it/v1alpha1",
			Kind:       "CommunitySchedule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: eav1alpha1.CommunityScheduleSpec{
			RoutingRules:     make(eav1alpha1.CommunitySourceRoutingRule),
			Allocations:      make(eav1alpha1.CommunityFunctionAllocation),
			AlgorithmService: "http://allocation-algorithm.default.svc.cluster.local:5000",
		},
	}
}

// NewCommunityController returns a new community controller deployment for a given community
func NewCommunityController(namespace, name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				ealabels.CommunityControllerDeploymentLabel: "",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"community": name,
					"app":       "community-controller",
				},
			},
			Replicas: pointer.Int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"community": name,
						"app":       "community-controller",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "controller",
							Image:           "systemautoscaler/community-controller:dev",
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								{
									Name:  "COMMUNITY_NAMESPACE",
									Value: namespace,
								},
								{
									Name:  "COMMUNITY_NAME",
									Value: name,
								},
							},
						},
					},
					ServiceAccountName:           "community-controller",
					AutomountServiceAccountToken: pointer.BoolPtr(true),
				},
			},
		},
	}
}
