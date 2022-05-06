package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lterrac/edge-autoscaler/pkg/community-controller/pkg/controller"
	ealabels "github.com/lterrac/edge-autoscaler/pkg/labels"
	openfaasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	eav1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const namespace = "e2e"
const timeout = 1 * time.Minute
const interval = 2 * time.Second

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
		CpuAllocations:   map[string]eav1alpha1.CommunityNodeAllocation{},
		CpuRoutingRules:  map[string]eav1alpha1.CommunityFunctionRoutingRule{},
		GpuAllocations:   map[string]eav1alpha1.CommunityNodeAllocation{},
		GpuRoutingRules:  map[string]eav1alpha1.CommunityFunctionRoutingRule{},
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
					return updatedCS.Spec.CpuRoutingRules != nil
				}
			}, 2*timeout, interval).Should(BeTrue())
		})

		It("Pod allocations are created", func() {
			Eventually(func() bool {
				updatedCS, err := eaClient.EdgeautoscalerV1alpha1().CommunitySchedules(communityNamespace).Get(ctx, communityName, metav1.GetOptions{})
				if err != nil {
					return false
				} else {
					return updatedCS.Spec.CpuAllocations != nil
				}
			}, 2*timeout, interval).Should(BeTrue())
		})

		It("Pods are created and scheduled", func() {
			Eventually(func() bool {
				allFound := true

				pods, err := kubeClient.CoreV1().Pods(communityNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())

				updatedCS, err := eaClient.EdgeautoscalerV1alpha1().CommunitySchedules(communityNamespace).Get(ctx, communityName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				for functionKey, nodes := range updatedCS.Spec.CpuAllocations {
					functionNamespace, functionName, err := cache.SplitMetaNamespaceKey(functionKey)
					Expect(err).ToNot(HaveOccurred())
					for node, ok := range nodes {
						found := false
						if ok {
							for _, pod := range pods.Items {
								podFunctionNamespace, okFunctionNamespace := pod.Labels[ealabels.FunctionNamespaceLabel]
								podFunctionName, okFunctionName := pod.Labels[ealabels.FunctionNameLabel]
								podFunctionNode, okFunctionNode := pod.Labels[ealabels.NodeLabel]
								podFunctionCommunity, okFunctionCommunity := pod.Labels[ealabels.CommunityLabel.WithNamespace(communityNamespace).String()]
								found = okFunctionNamespace && okFunctionName && okFunctionNode && okFunctionCommunity &&
									podFunctionNamespace == functionNamespace && podFunctionName == functionName && podFunctionNode == node && podFunctionCommunity == communityName
								if found {
									break
								}
							}
						}
						allFound = allFound && found
					}
				}
				return allFound
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
			NodeNames:       input.NodeNames,
			FunctionNames:   input.FunctionNames,
			CpuRoutingRules: routingRules,
			CpuAllocations:  allocations,
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

//TODO: ensure that pods are deleted whenever they should be deleted (when more than one are created for the same allocation)
//TODO: ensure that GPU pods are created
