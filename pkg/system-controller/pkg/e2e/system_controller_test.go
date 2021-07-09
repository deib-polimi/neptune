package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/emirpasic/gods/sets/hashset"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/system-controller/pkg/labels"
	"github.com/stretchr/testify/assert"

	eaapi "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const namespace = "e2e"
const slpaNamespace = "kube-system"
const timeout = 1 * time.Minute
const interval = 10 * time.Second

const slpaName = "slpa-algorithm"

var cc = &eaapi.CommunityConfiguration{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "example-slpa",
		Namespace: namespace,
	},
	TypeMeta: metav1.TypeMeta{
		Kind:       "CommunityConfiguration",
		APIVersion: eaapi.SchemeGroupVersion.Identifier(),
	},
	Spec: eaapi.CommunityConfigurationSpec{
		SlpaService:          "slpa.kube-system.svc.cluster.local:4567",
		CommunitySize:        3,
		MaximumDelay:         100,
		ProbabilityThreshold: 0,
		Iterations:           20,
	},
	Status: eaapi.CommunityConfigurationStatus{
		Communities: make(map[string][]string),
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

		It("has update the community configuration status", func() {
			updatedCC, err := eaClient.EdgeautoscalerV1alpha1().CommunityConfigurations(cc.Namespace).Get(ctx, cc.Name, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				var communities []string
				var exists bool
				if communities, exists = updatedCC.Status.Communities[cc.Namespace]; !exists {
					return false
				}
				return assert.ElementsMatch(&testing.T{}, generatedCommunities.Values(), communities)
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

func intPointer(number int) *int32 {
	val := int32(number)
	return &val
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
		Replicas: intPointer(1),
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
