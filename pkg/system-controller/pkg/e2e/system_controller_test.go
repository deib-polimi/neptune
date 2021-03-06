package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/emirpasic/gods/sets/hashset"

	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	"github.com/stretchr/testify/assert"

	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
				}
				return true
			}, 5*timeout, interval).Should(BeTrue())
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

				for _, node := range nodes.Items {
					labels := node.Labels

					// the master node is the only one that doesn't belong to a community
					if _, isMasterNode := labels[ealabels.MasterNodeLabel]; isMasterNode {
						continue
					}

					_, communityLabelExists = node.Labels[ealabels.CommunityLabel.WithNamespace(cc.Namespace).String()]

					if communityLabelExists {
						return false
					}
				}

				return true

			}, timeout, interval).Should(BeTrue())

		})
	})
})

// TODO: ensure that once a community is deleted, everything related to the community is deleted
