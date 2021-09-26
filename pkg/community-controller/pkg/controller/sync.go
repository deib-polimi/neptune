package controller

import (
	"context"
	"fmt"
	"github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	rand "math/rand"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *CommunityController) runScheduler(_ string) error {

	klog.Infof("Rescheduling community %s/%s", c.communityNamespace, c.communityName)

	// Retrieve the nodes
	nodeSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String(): c.communityName,
		})
	nodes, err := c.listers.NodeLister.List(nodeSelector)
	if err != nil {
		return fmt.Errorf("failed to retrieve nodes using selector %s with error: %s", nodeSelector, err)
	}

	// Retrieve the functions
	functions, err := c.listers.FunctionLister.Functions(c.communityNamespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to retrieve functions with error: %s", err)
	}

	input, err := NewSchedulingInput(nodes, functions)

	if err != nil {
		return fmt.Errorf("failed to create scheduling input with error: %s", err)
	}

	cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
	if err != nil {
		return fmt.Errorf("failed to get communitySchedule %s/%s with error: %s", c.communityNamespace, c.communityName, err)
	}

	scheduler := NewScheduler(cs.Spec.AlgorithmService)

	output, err := scheduler.Schedule(input)
	if err != nil {
		return fmt.Errorf("failed to compute scheduling output with error: %s", err)
	}

	cs = output.ToCommunitySchedule(cs)
	_, err = c.edgeAutoscalerClientSet.EdgeautoscalerV1alpha1().CommunitySchedules(c.communityNamespace).Update(context.TODO(), cs, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update community schedule %s/%s with error: %s", cs.Namespace, cs.Name, err)
	}
	return nil
}

// bind assigns a node to an unscheduled pod
func (c *CommunityController) bind(pod *corev1.Pod, nodeName string) error {
	klog.V(3).InfoS("Attempting to bind pod to node", "pod", klog.KObj(pod), "node", nodeName)
	binding := &corev1.Binding{
		ObjectMeta: metav1.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name, UID: pod.UID},
		Target:     corev1.ObjectReference{Kind: "Node", Name: nodeName},
	}
	err := c.kubernetesClientset.CoreV1().Pods(binding.Namespace).Bind(context.TODO(), binding, metav1.CreateOptions{})

	return err
}

// syncCommunityScheduleAllocation ensures that pods serving a certain function are created
// or deleted according to the community schedule allocation
func (c *CommunityController) syncCommunityScheduleAllocation(key string) error {

	klog.Infof("Syncing community schedule allocations %s", key)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	cs, err := c.listers.CommunitySchedules(namespace).Get(name)
	if err != nil {
		klog.Errorf("failed to retrieve community schedule %s/%s, with error %s", namespace, name, err)
		return err
	}

	functionKeys := cs.Spec.Allocations

	deleteSet := make([]*corev1.Pod, 0)
	createSet := make([]*corev1.Pod, 0)

	// Retrieve the pods managed by a certain community
	podSelector := labels.SelectorFromSet(
		map[string]string{
			ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String(): c.communityName,
		})
	pods, err := c.listers.PodLister.List(podSelector)
	if err != nil {
		klog.Errorf("can not list pod using selector %v, with error %v", podSelector, err)
		return err
	}

	// Find the pods that are correctly deployed
	// Pods which are not correctly deployed are appended in the deleteSet
	podsMap := make(map[string]map[string]*corev1.Pod)
	for _, pod := range pods {
		// TODO: should try to hand with && in order to make it more responsive, but it will create many pod at time
		if pod.Status.Phase == corev1.PodRunning || pod.DeletionTimestamp == nil {
			if fNamespace, ok := pod.Labels[ealabels.FunctionNamespaceLabel]; ok {
				if fName, ok := pod.Labels[ealabels.FunctionNameLabel]; ok {
					fKey := fmt.Sprintf("%s/%s", fNamespace, fName)
					if _, ok := podsMap[fKey]; !ok {
						podsMap[fKey] = make(map[string]*corev1.Pod)
					}
					if _, ok := podsMap[fKey][pod.Spec.NodeName]; ok {
						deleteSet = append(deleteSet, pod)
					} else {
						podsMap[fKey][pod.Spec.NodeName] = pod
					}
				}
			}
		}
	}

	// Compute the pod creation set
	// Pods which are correctly deployed are deleted from podsMap, in this way podsMap will contain only pods
	// which should be deleted
	for functionKey, nodes := range functionKeys {
		fNamespace, fName, err := cache.SplitMetaNamespaceKey(functionKey)
		if err != nil {
			klog.Errorf("invalid resource key: %s", functionKey)
			return err
		}
		function, err := c.listers.Functions(fNamespace).Get(fName)
		if err != nil {
			klog.Errorf("can not retrieve function %s/%s, with error %v", fNamespace, fName, err)
			return err
		}
		for node, ok := range nodes {
			if ok {
				if _, ok := podsMap[functionKey][node]; ok {
					delete(podsMap[functionKey], node)
				} else {
					pod := newPod(function, cs, node)
					createSet = append(createSet, pod)
				}
			}
		}
	}

	for _, nodes := range podsMap {
		for _, pod := range nodes {
			deleteSet = append(deleteSet, pod)
		}
	}

	for _, pod := range createSet {
		node := pod.Labels[ealabels.NodeLabel]
		pod, err = c.kubernetesClientset.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("failed to create pod %s/%s, with error %v", pod.Namespace, pod.Name, err)
			return err
		}
		err = c.bind(pod, node)
		if err != nil {
			klog.Errorf("failed to bind pod %s/%s to node %s, with error %v", pod.Namespace, pod.Name, node, err)
			return err
		}
	}

	for _, pod := range deleteSet {
		err = c.kubernetesClientset.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("failed to delete pod %s/%s, with error %v", pod.Namespace, pod.Name, err)
			return err
		}
	}

	return nil
}

// newPod creates a new Pod for a Function resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Function resource that 'owns' it.
func newPod(function *openfaasv1.Function, cs *v1alpha1.CommunitySchedule, node string) *corev1.Pod {

	envVars := makeEnvVars(function)

	resources, err := makeResources(function)
	if err != nil {
		klog.Warningf("Function %s resources parsing failed: %v",
			function.Spec.Name, err)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", function.Spec.Name, hash(8)),
			Annotations: function.Annotations,
			Namespace:   function.Namespace,
			Labels: map[string]string{
				ealabels.FunctionNamespaceLabel:                              function.Namespace,
				ealabels.FunctionNameLabel:                                   function.Name,
				ealabels.CommunityLabel.WithNamespace(cs.Namespace).String(): cs.Name,
				ealabels.NodeLabel:                                           node,
				"autoscaling": "vertical",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(function, schema.GroupVersionKind{
					Group:   openfaasv1.SchemeGroupVersion.Group,
					Version: openfaasv1.SchemeGroupVersion.Version,
					Kind:    "Function",
				}),
			},
		},
		Spec: corev1.PodSpec{
			SchedulerName: "edge-autoscaler",
			Containers: []corev1.Container{
				{
					Name:  function.Spec.Name,
					Image: function.Spec.Image,
					Ports: []corev1.ContainerPort{
						{ContainerPort: int32(8080), Protocol: corev1.ProtocolTCP},
					},
					ImagePullPolicy: corev1.PullAlways,
					Env:       envVars,
					Resources: *resources,
					// TODO: add probe with Function Factory
					//LivenessProbe:   probes.Liveness,
					//ReadinessProbe:  probes.Readiness,

				},
				{
					Name:  "http-metrics",
					Image: "systemautoscaler/http-metrics:0.1.0",
					Ports: []corev1.ContainerPort{
						{ContainerPort: int32(8000), Protocol: corev1.ProtocolTCP},
					},
					ImagePullPolicy: corev1.PullAlways,
					Env: []corev1.EnvVar{
						{
							Name:  "ADDRESS",
							Value: "localhost",
						},
						{
							Name:  "PORT",
							Value: "8080",
						},
						{
							Name:  "WINDOW_SIZE",
							Value: "30s",
						},
						{
							Name:  "WINDOW_GRANULARITY",
							Value: "1ms",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU: *resource.NewMilliQuantity(150, resource.BinarySI),
							corev1.ResourceMemory: *resource.NewQuantity(250000000, resource.BinarySI),
						},
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU: *resource.NewMilliQuantity(150, resource.BinarySI),
							corev1.ResourceMemory: *resource.NewQuantity(250000000, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	return pod

}

func makeEnvVars(function *openfaasv1.Function) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	if len(function.Spec.Handler) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "fprocess",
			Value: function.Spec.Handler,
		})
	}

	if function.Spec.Environment != nil {
		for k, v := range *function.Spec.Environment {
			envVars = append(envVars, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}

	return envVars
}

func makeResources(function *openfaasv1.Function) (*corev1.ResourceRequirements, error) {
	resources := &corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

	// Set Memory limits
	if function.Spec.Limits != nil && len(function.Spec.Limits.Memory) > 0 {
		qty, err := resource.ParseQuantity(function.Spec.Limits.Memory)
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceMemory] = qty
	}
	if function.Spec.Requests != nil && len(function.Spec.Requests.Memory) > 0 {
		qty, err := resource.ParseQuantity(function.Spec.Requests.Memory)
		if err != nil {
			return resources, err
		}
		resources.Requests[corev1.ResourceMemory] = qty
	}

	// Set CPU limits
	if function.Spec.Limits != nil && len(function.Spec.Limits.CPU) > 0 {
		qty, err := resource.ParseQuantity(function.Spec.Limits.CPU)
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceCPU] = qty
	}
	if function.Spec.Requests != nil && len(function.Spec.Requests.CPU) > 0 {
		qty, err := resource.ParseQuantity(function.Spec.Requests.CPU)
		if err != nil {
			return resources, err
		}
		resources.Requests[corev1.ResourceCPU] = qty
	}

	resources.Requests[corev1.ResourceCPU] = *resource.NewMilliQuantity(1000, resource.BinarySI)
	resources.Limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(1000, resource.BinarySI)

	resources.Requests[corev1.ResourceMemory] = *resource.NewQuantity(500000000, resource.BinarySI)
	resources.Limits[corev1.ResourceMemory] = *resource.NewQuantity(500000000, resource.BinarySI)

	return resources, nil
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func hash(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
