BASEDIR=$(dirname "$0")

# create cluster
# kind create cluster --config ./config/cluster-conf/kind.conf --image systemautoscaler/kindest-node:latest

# openfaas
# kubectl create ns openfaas
# kubectl create ns openfaas-fn
# helm install openfaas-kind openfaas/openfaas  --namespace openfaas --set basic_auth=false --set functionNamespace=openfaas-fn --set operator.create=true

# otherwise install with arkade
# arkade install openfaas --operator

# crd
# make manifests
# kubectl apply -f ./config/crd/bases

# generate admission webhook certificates and secret in tmpdir
chmod +x $BASEDIR/gen-certificates.sh
$BASEDIR/gen-certificates.sh --namespace kube-system --service function-deployment-custom-scheduler --secret function-deployment-custom-scheduler

# patch new mutitating webhook
chmod +x  $BASEDIR/patch-manifests.sh
$BASEDIR/patch-manifests.sh

# build docker image again
cd  $BASEDIR/..
make dev

# apply manifests
kubectl apply -f  $BASEDIR/deployment.yaml
kubectl apply -f  $BASEDIR/service.yaml
kubectl apply -f  $BASEDIR/admission-registration-subst.yaml
