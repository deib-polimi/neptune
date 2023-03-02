# Run NEPTUNE locally on KinD Cluster

# Requirements
- Docker (tested with version 23.0.1, cgroup driver 'systemd' and version '2')
- Kind (tested with version 0.13.0, installed with 'go install sigs.k8s.io/kind@v0.13.0')
- Kubectl

### Create KinD cluster

Use a custom KinD configuration and image to enable in-place vertical scaling.

```shell
kind create cluster --config config/cluster-conf/kind.conf --image systemautoscaler/kindest-node
```

### Install Custom Resources

```shell
kubectl apply -f config/crd/bases
kubectl apply -f config/cluster-conf/openfaas-fn-namespace.yaml
kubectl apply -f config/openfaas/openfaas-function.yaml
```

### Install Permission and RBAC

```shell
kubectl apply -f config/permissions
```

### Allow scheduling on kind-control-plane
```shell
kubectl taint node kind-control-plane node-role.kubernetes.io/master:NoSchedule-
kubectl label nodes kind-control-plane node-role.kubernetes.io/master=true --overwrite
```

### Install Controllers

```shell
kubectl apply -f config/deploy
```

Everything should work except for delay-discovery.

Enjoy!