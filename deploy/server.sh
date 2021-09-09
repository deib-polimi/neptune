#!/bin/sh

sudo cp -r ./k3s /usr/local/bin/
# chmod a+x ./k3s-install-script.sh
sudo snap install helm --classic
sudo snap install go --classic
curl -sfL https://get.k3s.io | INSTALL_K3S_SKIP_DOWNLOAD=true K3S_KUBECONFIG_MODE=666 INSTALL_K3S_VERSION=v1.19.13+k3s1 INSTALL_K3S_EXEC="server --flannel-backend=host-gw --node-name=k3s-master --docker --cluster-init --write-kubeconfig-mode  777 --kube-apiserver-arg=feature-gates=InPlacePodVerticalScaling=true"  sh -
mkdir /home/ubuntu/.kube
sudo mv /etc/rancher/k3s/k3s.yaml /home/ubuntu/.kube/config
sudo chmod u+rwx /home/ubuntu/.kube/config
sudo chmod go-rwx /home/ubuntu/.kube/config
sudo chown ubuntu /home/ubuntu/.kube/config
