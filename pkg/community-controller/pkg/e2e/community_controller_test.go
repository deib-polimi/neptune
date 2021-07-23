package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lterrac/edge-autoscaler/pkg/community-controller/pkg/controller"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	labels2 "k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"net/http"
	"time"

	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const namespace = "e2e"
const slpaNamespace = "kube-system"
const timeout = 1 * time.Minute
const interval = 2 * time.Second

const slpaName = "slpa-algorithm"
const ccName = "example-slpa"
const functionName = "function"

var communities = []string{
	"test-com1", "test-com2",
}

var cc = &eav1alpha1.CommunityConfiguration{
	ObjectMeta: metav1.ObjectMeta{
		Name:      ccName,
		Namespace: namespace,
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "CommunityConfiguration",
		APIVersion: eav1alpha1.SchemeGroupVersion.Identifier(),
	},
	Spec: eav1alpha1.CommunityConfigurationSpec{
		SlpaService:          "slpa.kube-system.svc.cluster.local:4567",
		CommunitySize:        3,
		MaximumDelay:         100,
		ProbabilityThreshold: 0,
		Iterations:           20,
	},
	Status: eav1alpha1.CommunityConfigurationStatus{
		Communities: communities,
	},
}

var cs = &eav1alpha1.CommunitySchedule{
	ObjectMeta: metav1.ObjectMeta{
		Name:      communityName,
		Namespace: communityNamespace,
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "CommunitySchedule",
		APIVersion: eav1alpha1.SchemeGroupVersion.Identifier(),
	},
	Spec: eav1alpha1.CommunityScheduleSpec{
		AlgorithmService: "http://localhost:12345/",
		Allocations:      map[string]eav1alpha1.CommunityNodeAllocation{},
		RoutingRules:     map[string]eav1alpha1.CommunityFunctionRoutingRule{},
	},
}

var _ = Describe("Community Controller", func() {

	ctx := context.Background()

	Context("With a Community Configuration and Community Schedule deployed inside the cluster", func() {

		It("Routing rules are created", func() {
			Eventually(func() bool {
				updatedCS, err := eaClient.EdgeautoscalerV1alpha1().CommunitySchedules(communityNamespace).Get(ctx, communityName, metav1.GetOptions{})
				if err != nil {
					return false
				} else {
					return updatedCS.Spec.RoutingRules != nil
				}
			}, 2*timeout, interval).Should(BeTrue())
		})

		It("Pod allocations are created", func() {
			Eventually(func() bool {
				updatedCS, err := eaClient.EdgeautoscalerV1alpha1().CommunitySchedules(communityNamespace).Get(ctx, communityName, metav1.GetOptions{})
				if err != nil {
					return false
				} else {
					return updatedCS.Spec.Allocations != nil
				}
			}, 2*timeout, interval).Should(BeTrue())
		})

		It("Schedules unscheduled pods", func() {

			communityReplicas := 0
			for _, node := range workerNodes {
				community, ok := node.Labels[ealabels.CommunityLabel.WithNamespace(namespace).String()]
				if ok {
					if community == communityName {
						pod := functionPod.DeepCopy()
						pod.Name = node.Name
						_, err := kubeClient.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
						Expect(err).ShouldNot(HaveOccurred())
						communityReplicas++
					}
				}
			}

			dp := functionDeployment.DeepCopy()
			dp.Spec.Replicas = pointer.Int32Ptr(int32(len(workerNodes)))

			_, err := kubeClient.AppsV1().Deployments(functionDeployment.Namespace).Create(ctx, functionDeployment, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				options := metav1.ListOptions{LabelSelector: labels2.SelectorFromSet(map[string]string{
					"faas_function": function.Spec.Name,
					"controller":    function.Name,
				}).String()}
				pods, err := kubeClient.CoreV1().Pods(functionDeployment.Namespace).List(ctx, options)
				if err != nil {
					klog.Info(err)
					return false
				}
				if len(pods.Items) != communityReplicas {
					return false
				}
				for _, pod := range pods.Items {
					if len(pod.Spec.NodeName) == 0 {
						return false
					}
				}
				return true
			}, 5*timeout, interval).Should(BeTrue())
		})

	})

})

func newFakeSchedulerServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var input controller.SchedulingInput

		// Try to decode the request body into the struct. If there is an error,
		// respond to the client with the error message and a 400 status code.
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Create routing rules
		routingRules := make(map[string]map[string]map[string]float64)
		allocations := make(map[string]map[string]bool)

		for _, source := range input.NodeNames {
			_, ok := routingRules[source]
			if !ok {
				routingRules[source] = make(map[string]map[string]float64)
				for _, f := range input.FunctionNames {
					_, ok = routingRules[source]
					if !ok {
						routingRules[source][f] = make(map[string]float64)
						for _, dest := range input.NodeNames {
							routingRules[source][f][dest] = 0.0
						}
					}
				}
			}
		}
		for _, f := range input.FunctionNames {
			_, ok := allocations[f]
			if !ok {
				allocations[f] = make(map[string]bool)
				for _, node := range input.NodeNames {
					allocations[f][node] = true
				}
			}
		}

		output := &controller.SchedulingOutput{
			NodeNames:     input.NodeNames,
			FunctionNames: input.FunctionNames,
			RoutingRules:  routingRules,
			Allocations:   allocations,
		}

		resp, err := json.Marshal(output)
		if err != nil {
			klog.Error(err)
		}
		if _, err := w.Write(resp); err != nil {
			klog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	})
	go func() {
		klog.Fatal(http.ListenAndServe(":12345", nil))
	}()
}

var slpaDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:      slpaName,
		Namespace: slpaNamespace,
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: appsv1.SchemeGroupVersion.Identifier(),
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": slpaName,
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": slpaName,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            slpaName,
						Image:           "systemautoscaler/slpa-rest:0.0.1",
						ImagePullPolicy: corev1.PullAlways,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 4567,
							},
						},
					},
				},
			},
		},
	},
}

var slpaService = &corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "slpa",
		Namespace: slpaNamespace,
		Labels: map[string]string{
			"app": slpaName,
		},
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: corev1.SchemeGroupVersion.Identifier(),
	},
	Spec: corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Port: 4567,
			},
		},
		Selector: map[string]string{
			"app": slpaName,
		},
	},
}

var function = &openfaasv1.Function{
	ObjectMeta: metav1.ObjectMeta{
		Name:      functionName,
		Namespace: namespace,
	},
	Spec: openfaasv1.FunctionSpec{
		Name:  functionName,
		Image: "ghcr.io/openfaas/figlet:latest",
		Labels: &map[string]string{
			"com.openfaas.scale.factor":          "20",
			"com.openfaas.scale.max":             "100",
			"com.openfaas.scale.min":             "1",
			"com.openfaas.scale.zero":            "false",
			"edgeautoscaler.polimi.it/scheduler": "edge-autoscaler",
		},
		Requests: &openfaasv1.FunctionResources{
			Memory: "1Mi",
		},
	},
}

var functionDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:      functionName,
		Namespace: namespace,
		Labels:    make(map[string]string),
		Annotations: map[string]string{
			"com.openfaas.function.spec": "something about openfaas",
		},
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: appsv1.SchemeGroupVersion.Identifier(),
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(0),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app":        functionName,
				"controller": functionName,
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app":           functionName,
					"controller":    functionName,
					"faas_function": functionName,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            functionName,
						Image:           "ghcr.io/openfaas/figlet:latest",
						ImagePullPolicy: corev1.PullAlways,
					},
				},
				SchedulerName: "edge-autoscaler",
			},
		},
	},
}

var functionPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      functionName,
		Namespace: namespace,
		Labels: map[string]string{
			"app":           functionName,
			"controller":    functionName,
			"faas_function": functionName,
		},
		Annotations: map[string]string{
			"com.openfaas.function.spec": "something about openfaas",
		},
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: appsv1.SchemeGroupVersion.Identifier(),
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            functionName,
				Image:           "ghcr.io/openfaas/figlet:latest",
				ImagePullPolicy: corev1.PullAlways,
			},
		},
		SchedulerName: "edge-autoscaler",
	},
}
