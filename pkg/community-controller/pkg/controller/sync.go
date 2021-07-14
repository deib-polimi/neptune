package controller

import (
	"context"
	"fmt"
	"github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/labels"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"strconv"
)

// ComputeDeploymentReplicas computes the new amount of replicas by taking in consideration all the amounts requested
// by the communities
func ComputeDeploymentReplicas(deployment *appsv1.Deployment, communityNamespace string, communities []string) (*int32, error) {
	instances := int32(0)
	for _, community := range communities {
		communityInstancesLabel := CommunityInstancesLabel.WithNamespace(communityNamespace).WithName(community).String()
		if val, ok := deployment.Labels[communityInstancesLabel]; ok {
			intVal, err := strconv.ParseInt(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("failed to parse community label %s for deployment %s/%s with error: %s", communityInstancesLabel, deployment.Namespace, deployment.Name, err)
			} else {
				instances += int32(intVal)
			}
		}
	}
	instances = instances * 5
	return &instances, nil
}

// syncDeploymentReplicas ensures that the amount of replicas assigned to a deployment
// the sum of the replicas assigned to all communities
func (c *CommunityController) syncDeploymentReplicas(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if namespace != c.communityNamespace {
		//return fmt.Errorf("deployment %s/%s should not be synced by this controller", namespace, name)
		return nil
	}

	if err != nil {
		return fmt.Errorf("invalid resource key: %s", key)
	}

	deployment, err := c.kubernetesClientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve deployment %s/%s with error: %s", namespace, name, err)
	}

	if deployment.ObjectMeta.Annotations == nil {
		klog.Infof("no annotation found for deployment %s/%s", namespace, name)
		return nil
	}
	_, ok := deployment.ObjectMeta.Annotations["com.openfaas.function.spec"]
	if !ok {
		return nil
	}

	ccList, err := c.listers.CommunityConfigurations(c.communityNamespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to retrieve community configuration %s/%s with error: %s", c.communityNamespace, c.communityName, err)
	}
	if len(ccList) != 1 {
		return fmt.Errorf("community configuration size for namespace %s should be 1", c.communityNamespace)
	}
	cc := ccList[0]

	replicas, err := ComputeDeploymentReplicas(deployment, cc.Namespace, cc.Status.Communities)
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = replicas
	klog.Info("replicas are %v", *replicas)

	_, err = c.kubernetesClientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment %s/%s with error: %s", namespace, name, err)
	}

	klog.Infof("updated deployment %s/%s", namespace, deployment.Name)
	klog.Infof("%v", deployment.Labels)
	return nil
}

// schedulePods schedules the pod as stated in the community schedule allocations
func (c *CommunityController) schedulePods(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	cs, err := c.listers.CommunitySchedules(namespace).Get(name)
	if err != nil {
		klog.Errorf("failed to retrieve community schedule %s/%s with error: %s", namespace, name, err)
		return nil
	}

	allocations := cs.Spec.Allocations

	for functionKey, nodeNames := range allocations {
		// Convert the namespace/name string into a distinct namespace and name
		namespace, name, err := cache.SplitMetaNamespaceKey(functionKey)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			return nil
		}

		function, err := c.listers.FunctionLister.Functions(namespace).Get(name)
		if err != nil {
			klog.Errorf("failed to retrieve function %s/%s with error: %s", namespace, name, err)
			return nil
		}

		pods, err := c.getPodsOfFunction(function)
		if err != nil {
			klog.Errorf("failed to retrieve pods associated with function %s/%s with error: %s", namespace, name, err)
			return nil
		}

		podsToBeDeleted := make([]*corev1.Pod, 0)
		podsToBeScheduled := make([]*corev1.Pod, 0)
		nodeMissingPod := make([]string, 0)
		nodeNamesSet := make(map[string]bool)

		// Create a node name map in order to facilitate the search
		for nodeName := range nodeNames {
			nodeNamesSet[nodeName] = true
		}
		for _, pod := range pods {
			// Look for pods which has not been scheduled yet
			if pod.Spec.NodeName == "" {
				podsToBeScheduled = append(podsToBeScheduled, pod)
			} else {
				// Look for pods which should be deleted
				_, ok := nodeNamesSet[pod.Spec.NodeName]
				if !ok {
					podsToBeDeleted = append(podsToBeDeleted, pod)
				} else {
					delete(nodeNamesSet, pod.Spec.NodeName)
				}
			}
		}
		// Find the set of nodes that don't have a pod deployed on but it should be
		for nodeName := range nodeNamesSet {
			nodeMissingPod = append(nodeMissingPod, nodeName)
		}

		// Find a node for all the pod that should be scheduled
		for i, pod := range podsToBeScheduled {
			if i < len(nodeMissingPod) {
				err = c.bind(pod, nodeMissingPod[i])
				if err != nil {
					klog.Errorf("failed to bind pod %s/%s to node %s with error: %s", pod.Namespace, pod.Name, nodeMissingPod[i], err)
				}
			}
		}

		for _, pod := range podsToBeDeleted {
			err = c.kubernetesClientset.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("failed to delete pod %s/%s with error: %s", pod.Namespace, pod.Name, err)
			}
		}
	}

	return nil
}

// TODO: the key is not used
func (c *CommunityController) runScheduler(key string) error {

	// Retrieve the nodes
	nodeSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String(): c.communityName,
		})
	nodes, err := c.listers.NodeLister.List(nodeSelector)
	//nodes, err := c.listers.NodeLister.List(labels.Everything())
	klog.Info(nodeSelector)
	if err != nil {
		return fmt.Errorf("failed to retrieve nodes using selector %s with error: %s", nodeSelector, err)
	}

	// Retrieve the functions
	functions, err := c.listers.FunctionLister.Functions(c.communityNamespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to retrieve functions with error: %s", err)
	}

	delays, err := c.getNodeDelays()
	if err != nil {
		return fmt.Errorf("failed to retrieve node delays with error: %s", err)
	}

	workloads, err := c.getWorkload()
	if err != nil {
		return fmt.Errorf("failed to retrieve node workloads with error: %s", err)
	}

	maxDelays, err := c.getMaxDelays()
	if err != nil {
		return fmt.Errorf("failed to retrieve max delays with error: %s", err)
	}

	input, err := NewSchedulingInput(nodes, functions, delays, workloads, maxDelays)
	if err != nil {
		return fmt.Errorf("failed to create scheduling input with error: %s", err)
	}

	scheduler := NewScheduler(schedulerServiceURL)

	output, err := scheduler.Schedule(input)
	if err != nil {
		return fmt.Errorf("failed to compute scheduling output with error: %s", err)
	}

	for _, function := range functions {
		deployment, err := c.kubernetesClientset.AppsV1().Deployments(function.Namespace).Get(context.TODO(), function.Spec.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to retrieve deployment %s/%s with error: %s", function.Namespace, function.Spec.Name, err)
		}
		deployment, err = scheduler.Apply(c.communityNamespace, c.communityName, output, function, deployment)
		if err != nil {
			return fmt.Errorf("failed to apply schedule for deployment %s/%s with error: %s", function.Namespace, function.Name, err)
		}
		_, err = c.kubernetesClientset.AppsV1().Deployments(deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update deployment %s/%s with error: %s", deployment.Namespace, deployment.Name, err)
		}
	}

	cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
	if err != nil {
		return fmt.Errorf("failed to get and create communitySchedule %s/%s with error: %s", c.communityNamespace, c.communityName, err)
	}
	cs = output.ToCommunitySchedule(cs)
	_, err = c.edgeAutoscalerClientSet.EdgeautoscalerV1alpha1().CommunitySchedules(c.communityNamespace).Update(context.TODO(), cs, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update community schedule %s/%s with error: %s", cs.Namespace, cs.Name, err)
	}
	return nil
}

func (c *CommunityController) getNodeDelays() ([][]int64, error) {

	// Retrieve the nodes
	nodeSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String(): c.communityName,
		})
	nodes, err := c.listers.NodeLister.List(nodeSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve nodes using selector %s with error: %s", nodeSelector, err)
	}

	delays := make([][]int64, len(nodes))
	for i := range delays {
		delays[i] = make([]int64, len(nodes))
	}
	return delays, nil

}

func (c *CommunityController) getWorkload() ([][]int64, error) {

	// Retrieve the nodes
	nodeSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String(): c.communityName,
		})
	nodes, err := c.listers.NodeLister.List(nodeSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve nodes using selector %s with error: %s", nodeSelector, err)
	}

	// Retrieve the functions
	functions, err := c.listers.FunctionLister.Functions(c.communityNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve functions with error: %s", err)
	}

	delays := make([][]int64, len(nodes))
	for i := range delays {
		delays[i] = make([]int64, len(functions))
	}
	return delays, nil

}

func (c *CommunityController) getMaxDelays() ([]int64, error) {

	// Retrieve the functions
	functions, err := c.listers.FunctionLister.Functions(c.communityNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve functions with error: %s", err)
	}

	delays := make([]int64, len(functions))

	return delays, nil

}

// getPodsOfFunction returns a list of pods which contains a given function
func (c *CommunityController) getPodsOfFunction(function *openfaasv1.Function) ([]*corev1.Pod, error) {
	selector := labels.SelectorFromSet(
		map[string]string{
			"controller":    function.Name,
			"faas_function": function.Spec.Name,
		})
	return c.listers.PodLister.Pods(function.Namespace).List(selector)
}

func (c *CommunityController) bind(pod *corev1.Pod, nodeName string) error {
	klog.V(3).InfoS("Attempting to bind pod to node", "pod", klog.KObj(pod), "node", nodeName)
	binding := &corev1.Binding{
		ObjectMeta: metav1.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name, UID: pod.UID},
		Target:     corev1.ObjectReference{Kind: "Node", Name: nodeName},
	}
	err := c.kubernetesClientset.CoreV1().Pods(binding.Namespace).Bind(context.TODO(), binding, metav1.CreateOptions{})

	return err
}

func (c *CommunityController) syncCommunitySchedule(key string) error {

	//Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	cc, err := c.listers.CommunityConfigurations(namespace).Get(name)
	if err != nil {
		return fmt.Errorf("failed to retrieve community configuration %s/%s with error: %s", c.communityNamespace, c.communityName, err)
	}

	if cc.Namespace == c.communityNamespace {
		for _, community := range cc.Status.Communities {
			if c.communityName == community {
				c.configurationName = cc.Name
				_, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
				//_, err := c.edgeAutoscalerClientSet.EdgeautoscalerV1alpha1().CommunitySchedules(c.communityNamespace).Get(context.TODO(),  c.communityName, metav1.GetOptions{})
				klog.Info(err)
				if err != nil {
					_, err = c.edgeAutoscalerClientSet.EdgeautoscalerV1alpha1().CommunitySchedules(c.communityNamespace).Create(context.TODO(), c.NewCommunitySchedule(), metav1.CreateOptions{})
					if err != nil {
						return fmt.Errorf("failed to create community schedule %s/%s with error: %s", c.communityNamespace, c.communityName, err)
					}
				}
			}
		}
	}
	return nil
}

func (c *CommunityController) NewCommunitySchedule() *v1alpha1.CommunitySchedule {
	return &v1alpha1.CommunitySchedule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "edgeautoscaler.polimi.it/v1alpha1",
			Kind:       "CommunitySchedule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.communityName,
			Namespace: c.communityNamespace},
		Spec: v1alpha1.CommunityScheduleSpec{
			RoutingRules: make(v1alpha1.CommunitySourceRoutingRule),
			Allocations: make(v1alpha1.CommunityFunctionAllocation),
		},
	}
}
