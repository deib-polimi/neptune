package e2e_test

import (
	"context"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	"k8s.io/utils/pointer"
	"strconv"
	"testing"
	"time"

	"github.com/emirpasic/gods/sets/hashset"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	"github.com/stretchr/testify/assert"

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
		Communities: []string{},
	},
}

var _ = Describe("System Controller", func() {
	Context("With a Community Configuration deployed inside the cluster", func() {
		var err error
		var nodes *corev1.NodeList
		generatedCommunities := hashset.New()

		ctx := context.Background()

		It("Waits for nodes to become ready", func() {
			// wait for nodes to become ready
			Eventually(func() bool {
				nodes, err = kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).ShouldNot(HaveOccurred())

				for _, node := range nodes.Items {
					for _, condition := range node.Status.Conditions {
						if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionFalse {
							return false
						}
					}
				}

				return true

			}, 6*timeout, interval).Should(BeTrue())
		})

		It("Creates the Community Configuration resource", func() {
			_, err = eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(namespace).Create(ctx, cc, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Assign labels to all worker nodes", func() {

			Eventually(func() bool {
				nodes, err = kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).ShouldNot(HaveOccurred())

				var communityLabelExists bool

				for _, node := range nodes.Items {
					labels := node.Labels

					// the master node is the only one that doesn't belong to a community
					if _, isMasterNode := labels[ealabels.MasterNodeLabel]; isMasterNode {
						continue
					}

					_, communityLabelExists = node.Labels[ealabels.CommunityLabel.WithNamespace(cc.Namespace).String()]
					// TODO: is right to return here? what about the other nodes?
					return communityLabelExists
				}

				return true

			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				communities := make(map[string][]*corev1.Node)
				for _, node := range nodes.Items {
					labels := node.Labels

					// the master node is the only one that doesn't belong to a community
					if _, isMasterNode := labels[ealabels.MasterNodeLabel]; isMasterNode {
						continue
					}

					communityName := node.Labels[ealabels.CommunityLabel.WithNamespace(cc.Namespace).String()]
					generatedCommunities.Add(communityName)
					communities[communityName] = append(communities[communityName], &node)
				}

				for _, community := range communities {
					Expect(len(community)).To(BeNumerically("<=", cc.Spec.CommunitySize))

					hasLeader := false

					for _, node := range community {
						if node.Labels[ealabels.CommunityRoleLabel.WithNamespace(namespace).String()] == ealabels.Leader.String() {
							hasLeader = true
						}
					}

					if !hasLeader {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("Updates the replicas of function deployments based on the one assigned to theirs community one", func() {
			_, err = openfaasClient.OpenfaasV1().Functions(function.Namespace).Create(ctx, function, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			var cc *eav1alpha1.CommunityConfiguration

			// Eventually communities are found and generated
			Eventually(func() bool {
				cc, err = eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(namespace).Get(ctx, ccName, metav1.GetOptions{})
				if err != nil {
					return false
				} else {
					return len(cc.Status.Communities) > 0
				}
			}, timeout, interval).Should(BeTrue())

			for _, community := range cc.Status.Communities {
				functionDeployment.Labels[ealabels.CommunityInstancesLabel.WithNamespace(functionDeployment.Namespace).WithName(community).String()] = "3"
			}

			_, err = kubeClient.AppsV1().Deployments(functionDeployment.Namespace).Create(ctx, functionDeployment, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// Eventually the deployment instances are set as the sum of the community instances
			Eventually(func() bool {
				instances := int32(0)
				dp, err := kubeClient.AppsV1().Deployments(function.Namespace).Get(ctx, function.Spec.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				cc, err := eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(namespace).Get(ctx, ccName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				for _, community := range cc.Status.Communities {
					instance, ok := dp.ObjectMeta.Labels[ealabels.CommunityInstancesLabel.WithNamespace(function.Namespace).WithName(community).String()]
					if !ok {
						return false
					}
					intVal, err := strconv.ParseInt(instance, 10, 32)
					if err != nil {
						return false
					} else {
						instances += int32(intVal)
					}
				}
				return *dp.Spec.Replicas == instances
			}, timeout, interval).Should(BeTrue())
		})

		It("Creates community schedule for each community", func() {

			var cc *eav1alpha1.CommunityConfiguration

			// Eventually communities are found and generated
			Eventually(func() bool {
				cc, err = eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(namespace).Get(ctx, ccName, metav1.GetOptions{})
				if err != nil {
					return false
				} else {
					return len(cc.Status.Communities) > 0
				}
			}, timeout, interval).Should(BeTrue())

			// Eventually all community schedule are created
			Eventually(func() bool {
				allFound := true
				for _, community := range cc.Status.Communities {
					cs, err := eaClient.EdgeautoscalerV1alpha1().CommunitySchedules(namespace).Get(ctx, community, metav1.GetOptions{})
					if err != nil {
						return false
					}
					allFound = allFound && cs.Namespace == namespace && cs.Name == community
				}
				return allFound
			}, timeout, interval).Should(BeTrue())

		})

		It("Creates community controller for each community", func() {

			var cc *eav1alpha1.CommunityConfiguration

			// Eventually communities are found and generated
			Eventually(func() bool {
				cc, err = eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(namespace).Get(ctx, ccName, metav1.GetOptions{})
				if err != nil {
					return false
				} else {
					return len(cc.Status.Communities) > 0
				}
			}, timeout, interval).Should(BeTrue())

			// Eventually all community schedule are created
			Eventually(func() bool {
				allFound := true
				for _, community := range cc.Status.Communities {
					dp, err := kubeClient.AppsV1().Deployments(namespace).Get(ctx, community, metav1.GetOptions{})
					if err != nil {
						return false
					}
					allFound = allFound && dp.Namespace == namespace && dp.Name == community
				}
				return allFound
			}, timeout, interval).Should(BeTrue())

		})

		It("Has update the community configuration status", func() {
			updatedCC, err := eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(cc.Namespace).Get(ctx, cc.Name, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				if len(updatedCC.Status.Communities) == 0 {
					return false
				}
				return assert.ElementsMatch(&testing.T{}, generatedCommunities.Values(), updatedCC.Status.Communities)
			}, timeout, interval).Should(BeTrue())
		})

		It("Removes the labels when the CommunityConfiguration does not exist anymore", func() {
			err = eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(namespace).Delete(ctx, cc.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				nodes, err = kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).ShouldNot(HaveOccurred())

				var communityLabelExists bool
				var communityRoleLabelExists bool

				for _, node := range nodes.Items {
					labels := node.Labels

					// the master node is the only one that doesn't belong to a community
					if _, isMasterNode := labels[ealabels.MasterNodeLabel]; isMasterNode {
						continue
					}

					_, communityLabelExists = node.Labels[ealabels.CommunityLabel.WithNamespace(cc.Namespace).String()]
					_, communityRoleLabelExists = node.Labels[ealabels.CommunityRoleLabel.WithNamespace(cc.Namespace).String()]

					if communityLabelExists || communityRoleLabelExists {
						return false
					}
				}

				return true

			}, timeout, interval).Should(BeTrue())

		})
	})
})

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
			},
		},
	},
}
