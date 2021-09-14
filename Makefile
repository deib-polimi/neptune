MAKEFLAGS += --no-print-directory
COMPONENTS = community-controller system-controller edge-scheduler function-deployment-webhook dispatcher

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

CRD_OPTIONS ?= "crd:trivialVersions=true"

.PHONY: all build cluster clean coverage controller-gen e2e fmt install install-crds install-rbac manifests release test vet

all: build coverage clean manifests test

build:
	$(call action, build)

coverage:
	$(call action, coverage)

clean:
	$(call action, clean)

test:
	$(call action, test)

install: install-crds install-rbac

install-crds: manifests
	@echo "install CRDs manifests"
	@kubectl apply -f config/crd/bases

e2e: install
	@echo "run e2e tests"
	@kubectl apply -f ./config/cluster-conf/e2e-namespace.yaml
	@kubectl apply -f ./config/openfaas
	$(call action, e2e)
	@kubectl delete -f ./config/cluster-conf/e2e-namespace.yaml

install-rbac:
	@echo "install RBAC"
	@kubectl apply -f ./config/cluster-conf/openfaas-fn-namespace.yaml
	@kubectl apply -f config/permissions

metric-db:
	@cd ./config/metric-db
	@docker build -t systemautoscaler/database .
	@docker push -t systemautoscaler/database
# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=edgeautoscaler-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases crd:crdVersions={v1}
	@rm config/edgeautoscaler.polimi.it_communityschedules.yaml
	@rm config/edgeautoscaler.polimi.it_communityconfigurations.yaml

controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif


define action
	@for c in $(COMPONENTS); \
		do \
		$(MAKE) $(1) -C pkg/$$c; \
    done
endef
