# create cluster
kind create cluster --config ./config/cluster-conf/kind.conf --image systemautoscaler/kindest-node:latest

# openfaas
kubectl create ns openfaas
kubectl create ns openfaas-fn
helm install openfaas-kind openfaas/openfaas  --namespace openfaas --set basic_auth=false --set functionNamespace=openfaas-fn --set operator.create=true

# otherwise install with arkade
# arkade install openfaas --operator

# crd
make manifests
kubectl apply -f ./config/crd/bases

# generate admission webhook certificates and secret in tmpdir
chmod +x ./pkg/function-deployment-webhook/deploy/gen-certificates.sh 
./pkg/function-deployment-webhook/deploy/gen-certificates.sh --namespace openfaas-fn --service function-deployment-custom-scheduler --secret function-deployment-custom-scheduler

# patch new mutitating webhook
chmod +x ./pkg/function-deployment-webhook/deploy/patch-manifests.sh
./pkg/function-deployment-webhook/deploy/patch-manifests.sh

# build docker image again
cd ./pkg/function-deployment-webhook
make dev

# apply manifests
kubectl apply -f ./deploy/deployment.yaml
kubectl apply -f ./deploy/service.yaml
kubectl apply -f ./deploy/admission-registration-subst.yaml
