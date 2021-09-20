#!/bin/sh
set -e

USER=ubuntu
# HOST="18.222.168.97" #master
# HOST="52.14.143.121" #worker
for HOST in  "3.141.21.136" "18.116.12.12" "3.22.70.178" "18.188.192.155"
do
PREFIX_SSH="ssh -i kosmos.pem ${USER}@${HOST}"
PREFIX_SCP="scp -i kosmos.pem"

sudo chmod 400 kosmos.pem
sudo chmod 777 k3s
${PREFIX_SSH} sudo apt update
${PREFIX_SSH} sudo apt upgrade -y
${PREFIX_SSH} sudo apt install -y docker.io
${PREFIX_SSH} sudo systemctl start docker
${PREFIX_SSH} sudo systemctl enable docker
${PREFIX_SCP} -C -r $PWD ${USER}@${HOST}:/home/${USER}

done
# K3S_KUBECONFIG_MODE="666"
# ${PREFIX_SSH} K3S_KUBECONFIG_MODE=666 INSTALL_K3S_VERSION=v1.19.13+k3s1 INSTALL_K3S_EXEC="server --write-kubeconfig-mode  777 --kube-apiserver-arg=feature-gates=InPlacePodVerticalScaling=true" ./k3s-install-script.sh

# source /home/ubuntu/.ssh/environment
# sudo chmod 777 k3s-install-script.sh
# INSTALL_K3S_SKIP_DOWNLOAD=true K3S_KUBECONFIG_MODE=666 INSTALL_K3S_VERSION=v1.19.13+k3s1 INSTALL_K3S_EXEC="server --write-kubeconfig-mode  777 --kube-apiserver-arg=feature-gates=InPlacePodVerticalScaling=true" ./k3s-install-script.sh
# INSTALL_K3S_SKIP_DOWNLOAD=true K3S_KUBECONFIG_MODE=666 INSTALL_K3S_VERSION=v1.19.13+k3s1 K3S_TOKEN=K10b0334de665412a5658774fba517943ad895aa0ca33a277fbc67219f1d2bfb5ad::server:b5311ba48a90bf16c943095376ceae7a K3S_URL=https://172.31.16.28:6443 ./k3s-install-script.sh
