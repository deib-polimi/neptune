#!/bin/bash

ROOT=$(cd $(dirname $0)/../../; pwd)

set -o errexit
set -o nounset
set -o pipefail

export CA_BUNDLE=$(kubectl get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 | tr -d '\n')

#if command -v envsubst >/dev/null 2>&1; then
if command -v envsubst; then
envsubst < $(dirname $0)/admission-registration.yaml > $(dirname $0)/admission-registration-subst.yaml
else
sed -e "s|\${CA_BUNDLE}|${CA_BUNDLE}|g"
fi
