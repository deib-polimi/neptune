package controller

import (
	"context"
	"fmt"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// TODO: the key is not used
func (c *CommunityController) runScheduler(key string) error {

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

	cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
	if err != nil {
		return fmt.Errorf("failed to get communitySchedule %s/%s with error: %s", c.communityNamespace, c.communityName, err)
	}

	scheduler := NewScheduler(cs.Spec.AlgorithmService)

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

// syncCommunitySchedule ensures that the community schedule allocation is consistent
// by deleting the pods which are not necessary anymore
func (c *CommunityController) syncCommunitySchedule(key string) error {

	klog.Infof("Syncing community schedule %s", key)

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

	functions, err := c.listers.Functions(namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("failed to list functions in namespace %s, with error %s", namespace, err)
		return err
	}

	for _, function := range functions {

		fkey := function.Namespace + "/" + function.Name
		if _, ok := cs.Spec.Allocations[fkey]; !ok {
			klog.Errorf("failed to find function %s/%s in community schedule allocations", function.Namespace, function.Name)
			return fmt.Errorf("failed to find function %s/%s in community schedule allocations", function.Namespace, function.Name)
		}

		pods, err := c.GetPodsOfFunction(function)
		if err != nil {
			klog.Errorf("failed to retrieve pods of function %s/%s, with error %s", function.Namespace, function.Name, err)
			return fmt.Errorf("failed to retrieve pods of function %s/%s, with error %s", function.Namespace, function.Name, err)
		}

		for _, pod := range pods {
			// If the pod has been scheduled but on a node which is not present in the community schedule allocation
			if len(pod.Spec.NodeName) != 0 {
				if community, ok := pod.ObjectMeta.Labels[ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String()]; ok && community == c.communityName {
					if _, ok := cs.Spec.Allocations[fkey][pod.Spec.NodeName]; !ok {
						klog.Info(cs.Spec.Allocations)
						klog.Info(pod.Spec.NodeName)
						err := c.kubernetesClientset.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
						if err != nil {
							klog.Errorf("failed to delete pod %s/%s, with error %s", pod.Namespace, pod.Name, err)
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// schedulePod finds a node for an unscheduled pod
// the node of the pod is given by the community schedule allocation
func (c *CommunityController) schedulePod(key string) error {

	klog.Infof("Scheduling pod %s", key)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	pod, err := c.listers.Pods(namespace).Get(name)
	if err != nil {
		klog.Errorf("failed to retrieve pod %s/%s, with error %s", namespace, name, err)
		return err
	}

	// Check if pod has been already scheduled
	if len(pod.Spec.NodeName) != 0 {
		klog.Errorf("pod %s/%s has already been scheduled", namespace, name)
		return nil
	}

	function, err := c.GetFunctionOfPod(pod)
	if err != nil {
		klog.Errorf("failed to retrieve function of pod %s/%s, with error %s", namespace, name, err)
		return err
	}

	cs, err := c.listers.CommunitySchedules(c.communityNamespace).Get(c.communityName)
	if err != nil {
		klog.Errorf("failed to retrieve community schedule %s/%s, with error %s", c.communityNamespace, c.communityName, err)
		return err
	}

	otherPods, err := c.GetPodsOfFunction(function)
	if err != nil {
		klog.Errorf("failed to retrieve pods of function %s/%s, with error %s", function.Namespace, function.Name, err)
		return err
	}

	busyNodes := make(map[string]bool)
	for _, otherPod := range otherPods {
		busyNodes[otherPod.Spec.NodeName] = true
	}

	if _, ok := cs.Spec.Allocations[function.Namespace+"/"+function.Name]; ok {
		for nodeName, v := range cs.Spec.Allocations[function.Namespace+"/"+function.Name] {
			if v {
				if _, ok = busyNodes[nodeName]; !ok {
					err = c.bind(pod, nodeName)
					if err != nil {
						klog.Errorf("failed to bind pod %s/%s to node %s, with error %s", namespace, name, nodeName, err)
					}
					pod.ObjectMeta.Labels[ealabels.CommunityLabel.WithNamespace(c.communityNamespace).String()] = c.communityName
					_, err := c.kubernetesClientset.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
					if err != nil {
						klog.Errorf("failed to update pod %s/%s to node %s, with error %s", namespace, name, nodeName, err)
					}
					break
				}
			}
		}
	} else {
		klog.Errorf("failed to find function %s/%s in community schedule allocations", function.Namespace, function.Name)
		return fmt.Errorf("failed to find function %s/%s in community schedule allocations", function.Namespace, function.Name)
	}

	klog.Errorf("failed to find a node for pod %s/%s", function.Namespace, function.Name)
	return fmt.Errorf("failed to find a node for pod %s/%s", function.Namespace, function.Name)
}
