package controller

import (
	"context"
	"fmt"
	rand "math/rand"
	"strings"

	"github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	dispatcher "github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/controller"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	HttpMetricsImage   = "systemautoscaler/http-metrics"
	HttpMetrics        = "http-metrics"
	HttpMetricsVersion = "dev"
	HttpMetricsPort    = 8080
	HttpMetricsCpu     = 100
	HttpMetricsMemory  = 200000000
	DefaultAppPort     = "8080"
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

	// Retrieve the pods
	pods, err := c.listers.PodLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to retrieve pods with error: %s", err)
	}

	communitySchedule, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
	if err != nil {
		return fmt.Errorf("failed to retrieve community schedule with error: %s", err)
	}

	input, err := NewSchedulingInput(communitySchedule.Namespace, communitySchedule.Name, nodes, functions, pods, communitySchedule.Spec.CpuAllocations, communitySchedule.Spec.GpuAllocations)

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

	cpu_pods := []*corev1.Pod{}
	gpu_pods := []*corev1.Pod{}

	for _, p := range pods {
		if _, ok := p.Labels[ealabels.GpuFunctionLabel]; ok {
			gpu_pods = append(gpu_pods, p)
		} else {
			cpu_pods = append(cpu_pods, p)
		}
	}

	err = c.sync(cs, gpu_pods, cs.Spec.GpuAllocations, true)

	if err != nil {
		return fmt.Errorf("failed to sync community schedule allocation for GPU pods with error: %s", err)
	}

	err = c.sync(cs, cpu_pods, cs.Spec.CpuAllocations, false)

	if err != nil {
		return fmt.Errorf("failed to sync community schedule allocation for CPU pods with error: %s", err)
	}

	return nil
}

// sync is used to sync both GPU and CPU pods. The switch strategy is determined with a bool since
// most of the code is shared between the two procedures
func (c *CommunityController) sync(cs *v1alpha1.CommunitySchedule, pods []*corev1.Pod, functionKeys v1alpha1.CommunityFunctionAllocation, gpu bool) error {

	var err error

	// used to delete duplicated pods
	deleteSet := make([]*corev1.Pod, 0)
	createSet := make([]*corev1.Pod, 0)

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
	for _, pod := range createSet {
		klog.Infof("creating pod %s", pod.Name)
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
		for nodeName, ok := range nodes {
			if ok {
				if _, ok := podsMap[functionKey][nodeName]; ok {
					delete(podsMap[functionKey], nodeName)
					continue
				}

				node, err := c.listers.NodeLister.Get(nodeName)

				if err != nil {
					klog.Errorf("can not retrieve node %s, with error %v", nodeName, err)
					return err
				}

				var pod *corev1.Pod
				if _, ok := function.Labels[ealabels.GpuFunctionLabel]; ok && gpu {
					pod = newGPUPod(function, cs, node)

				} else {
					pod = newCPUPod(function, cs, node)
				}

				createSet = append(createSet, pod)

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

	for _, pod := range createSet {
		pod, err = c.kubernetesClientset.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get pod %s/%s, with error %v", pod.Namespace, pod.Name, err)
			return err
		}

		if !dispatcher.IsPodReady(pod) {
			deleteSet = nil
			break
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

// newCPUPod creates a new Pod for a Function resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Function resource that 'owns' it.
func newCPUPod(function *openfaasv1.Function, cs *v1alpha1.CommunitySchedule, node *corev1.Node) *corev1.Pod {

	envVars := makeEnvVars(function)

	resources, err := makeResources(function)
	if err != nil {
		klog.Warningf("Function %s resources parsing failed: %v",
			function.Spec.Name, err)
	}

	image := function.Spec.Image

	// pull different image for inference pods running on cpu
	if _, ok := function.Labels[ealabels.GpuFunctionLabel]; ok {
		image = strings.ReplaceAll(image, "-gpu", "")
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
				ealabels.NodeLabel:                                           node.Name,
				"autoscaling":                                                "vertical",
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
					Image: image,
					Ports: []corev1.ContainerPort{
						{ContainerPort: int32(HttpMetricsPort), Protocol: corev1.ProtocolTCP},
					},
					ImagePullPolicy: corev1.PullAlways,
					Env:             envVars,
					Resources:       *resources,
					// TODO: add probe with Function Factory
					//LivenessProbe:   probes.Liveness,
					//ReadinessProbe:  probes.Readiness,
				},
				{
					Name:  "http-metrics",
					Image: fmt.Sprintf("%s:%s", HttpMetricsImage, HttpMetricsVersion),
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
							Name:  "APP_PORT",
							Value: DefaultAppPort,
						},
						{
							Name:  "WINDOW_SIZE",
							Value: "30s",
						},
						{
							Name:  "WINDOW_GRANULARITY",
							Value: "1ms",
						},
						{
							Name:  "NODE",
							Value: node.Name,
						},
						{
							Name:  "FUNCTION",
							Value: function.Name,
						},
						{
							Name:  "NAMESPACE",
							Value: function.Namespace,
						},
						{
							Name:  "COMMUNITY",
							Value: cs.Name,
						},
						{
							Name:  "GPU",
							Value: "false",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(HttpMetricsCpu, resource.BinarySI),
							corev1.ResourceMemory: *resource.NewQuantity(HttpMetricsMemory, resource.BinarySI),
						},
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(HttpMetricsCpu, resource.BinarySI),
							corev1.ResourceMemory: *resource.NewQuantity(HttpMetricsMemory, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	return pod

}

// newGPUPod creates a new Pod for a Function resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Function resource that 'owns' it.
// the bool is used to handle pod replicas running on CPU
func newGPUPod(function *openfaasv1.Function, cs *v1alpha1.CommunitySchedule, node *corev1.Node) *corev1.Pod {

	envVars := makeEnvVars(function)

	resources, err := makeResources(function)
	if err != nil {
		klog.Warningf("Function %s resources parsing failed: %v",
			function.Spec.Name, err)
	}

	functionGPUMemory, err := resource.ParseQuantity((*function.Spec.Labels)[ealabels.GpuFunctionMemoryLabel])

	if err != nil {
		klog.Warningf("Function %s gpu memory parsing failed: %v",
			function.Spec.Name, err)
	}

	nodeGPUMemory, err := resource.ParseQuantity(node.Labels[ealabels.GpuNodeMemoryLabel])

	if err != nil {
		klog.Warningf("Function %s gpu memory parsing failed: %v",
			function.Spec.Name, err)
	}

	gpuMemoryFraction := float64(functionGPUMemory.Value()) / float64(nodeGPUMemory.Value())

	if _, ok := (*function.Spec.Labels)[ealabels.GpuFunctionVGPU]; !ok {
		klog.Warningf("Function %s has no virtual gpu set: %v",
			function.Spec.Name, err)
	}

	vgpu, err := resource.ParseQuantity((*function.Spec.Labels)[ealabels.GpuFunctionVGPU])

	if err != nil {
		klog.Warningf("Function %s vgpu resources parsing failed: %v",
			function.Spec.Name, err)
	}

	resources.Limits[corev1.ResourceName(ealabels.GpuFunctionVGPU)] = vgpu
	resources.Requests[corev1.ResourceName(ealabels.GpuFunctionVGPU)] = vgpu

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s-gpu", function.Spec.Name, hash(8)),
			Annotations: function.Annotations,
			Namespace:   function.Namespace,
			Labels: map[string]string{
				ealabels.FunctionNamespaceLabel:                              function.Namespace,
				ealabels.FunctionNameLabel:                                   function.Name,
				ealabels.CommunityLabel.WithNamespace(cs.Namespace).String(): cs.Name,
				ealabels.NodeLabel:                                           node.Name,
				ealabels.GpuFunctionLabel:                                    "",
				"autoscaling":                                                "vertical",
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
			HostIPC:       true,
			Volumes: []corev1.Volume{
				{
					Name: "nvidia-mps",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/tmp/nvidia-mps",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  function.Spec.Name,
					Image: function.Spec.Image,
					Ports: []corev1.ContainerPort{
						// default tensorflow serving REST api port si 8080
						{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
					},
					ImagePullPolicy: corev1.PullAlways,
					Env:             envVars,
					Resources:       *resources,
					Args:            []string{fmt.Sprintf("--per_process_gpu_memory_fraction=%.2f", gpuMemoryFraction)},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "nvidia-mps",
							MountPath: "/tmp/nvidia-mps",
						},
					},
					// TODO: add probe with Function Factory
					//LivenessProbe:   probes.Liveness,
					//ReadinessProbe:  probes.Readiness,
				},
				{
					Name:  "http-metrics",
					Image: fmt.Sprintf("%s:%s", HttpMetricsImage, HttpMetricsVersion),
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
							Name:  "APP_PORT",
							Value: DefaultAppPort,
						},
						{
							Name:  "WINDOW_SIZE",
							Value: "30s",
						},
						{
							Name:  "WINDOW_GRANULARITY",
							Value: "1ms",
						},
						{
							Name:  "NODE",
							Value: node.Name,
						},
						{
							Name:  "FUNCTION",
							Value: function.Name,
						},
						{
							Name:  "NAMESPACE",
							Value: function.Namespace,
						},
						{
							Name:  "COMMUNITY",
							Value: cs.Name,
						},
						{
							Name:  "GPU",
							Value: "true",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(HttpMetricsCpu, resource.BinarySI),
							corev1.ResourceMemory: *resource.NewQuantity(HttpMetricsMemory, resource.BinarySI),
						},
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(HttpMetricsCpu, resource.BinarySI),
							corev1.ResourceMemory: *resource.NewQuantity(HttpMetricsMemory, resource.BinarySI),
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
	if function.Spec.Limits != nil && len(function.Spec.Limits.Memory) > 0 &&
		function.Spec.Requests != nil && len(function.Spec.Requests.Memory) > 0 {
		limit, err := resource.ParseQuantity(function.Spec.Limits.Memory)
		if err != nil {
			return resources, err
		}
		request, err := resource.ParseQuantity(function.Spec.Requests.Memory)
		if err != nil {
			return resources, err
		}
		if limit.MilliValue() != request.MilliValue() {
			return resources, fmt.Errorf("function %s/%s must have same memory resource requests and limits", function.Namespace, function.Name)
		}
		resources.Requests[corev1.ResourceMemory] = request
		resources.Limits[corev1.ResourceMemory] = limit
	} else {
		return resources, fmt.Errorf("function %s/%s must have memory resource requests and limits", function.Namespace, function.Name)
	}

	// Set CPU limits
	if function.Spec.Limits != nil && len(function.Spec.Limits.CPU) > 0 &&
		function.Spec.Requests != nil && len(function.Spec.Requests.CPU) > 0 {
		limit, err := resource.ParseQuantity(function.Spec.Limits.CPU)
		if err != nil {
			return resources, err
		}
		request, err := resource.ParseQuantity(function.Spec.Requests.CPU)
		if err != nil {
			return resources, err
		}
		if limit.MilliValue() != request.MilliValue() {
			return resources, fmt.Errorf("function %s/%s must have same cpu resource requests and limits", function.Namespace, function.Name)
		}
		resources.Requests[corev1.ResourceCPU] = request
		resources.Limits[corev1.ResourceCPU] = limit
	} else {
		return resources, fmt.Errorf("function %s/%s must have cpu resource requests and limits", function.Namespace, function.Name)
	}

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
