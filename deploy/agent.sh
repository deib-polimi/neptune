#!/bin/sh

sudo cp ./k3s /usr/local/bin/
# chmod a+x ./k3s-install-script.sh
curl -sfL https://get.k3s.io | INSTALL_K3S_SKIP_DOWNLOAD=true K3S_KUBECONFIG_MODE=666 INSTALL_K3S_VERSION=v1.19.13+k3s1 K3S_TOKEN=$1 K3S_URL=$2 INSTALL_K3S_EXEC="--node-name=k3s-worker-1 --docker " sh -
