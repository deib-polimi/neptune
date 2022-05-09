![pipelines](https://github.com/lterrac/system-autoscaler/workflows/base-pipeline/badge.svg)

# NEPTUNE
<p align="center">
  <img width="100%" src="https://i.imgur.com/tm9mSuM.png" alt="Politecnico di Milano" />
</p>

## Overview

NEPTUNE is a comprehensive framework, developed at the Politecnico di Milano, for the runtime management of large-scale edge applications that exploits placement, routing, network delays, and CPU/GPU interplay in a coordinated way to allow for the concurrent execution of edge applications that meet user-set response times.

NEPTUNE implementation run on top of Kubernetes or Kubernetes-compatible orchestrator as a set of Kubernetes Controllers and Custom Resources.

## Controllers

* **System Controller**: `SystemController` is the component that splits large edge topologies into smaller, closely-located and indenpendet communities.     Communities are found using an external function that, given a set of nodes and the latencies between all nodes, returns a set of communities and the set of nodes assigned to each community. 
    The base implementation of the `SystemController` adopts SLPA (Speaker-Listener Label Propagation Algorithm, Xie et al., 2019). The implementation of the SLPA algorithm can be found [here](https://github.com/deib-polimi/paps-SLPA).
    The `SystemController` takes in input `CommunityConfigurations` Custom Resources and it splits the network of nodes according to `CommunityConfigurations`.
    A network of nodes is split into communities by assigning to each node a label that denote to which community the node belongs to.

* **Community Controller**: `CommunityController` is the component that schedules functions on nodes and provides routing policies for smart requests routing.
    As in the `SystemController`, function schedules and routing policies are found using an external function that requires the information about the topology and the set of functions to be deployed on top of the nodes. 
    The `CommunityController` relies on solving a two-step mixed integer linear programming problem. The solver can be found [here](https://github.com/deib-polimi/neptune-mip).
    The `CommunityController` is composed by two control loops: the first one updates `CommunitySchedule` using the two-step mixed integer linear programming, and the second one tries to reconcile the actual function schedule and the one specified in `CommunitySchedule`.

* **Node Controller**: `NodeController` is the component that vertical scales function in order to meet user-set response times. The node controller has been implemented in another project named Kosmos. The project can be found [here](https://github.com/deib-polimi/kosmos).

* **Request Dispatcher**: `RequestDispacher` is the component that routes requests to function instances according to the routing policies specified in `CommunitySchedule`. The requests dispatcher works only with HTTP and HTTPS messages.

* **Monitoring**: `Monitoring` is the component that monitors and collect informations about resource consumption and network performance inside the cluster.

* **Metrics Database**: `MetricsDatabase` is a SQL database that stores data. It has been chosen to use TimeScaleDB, but since it has an interface PostGresQL-compatible, it's also possible to use PSQL. 

## Resources

* **Community Configuration**: `CommunityConfiguration` defines the properties of each community inside the cluster. An example is:
```
apiVersion: edgeautoscaler.polimi.it/v1alpha1
kind: CommunityConfiguration
metadata:
  name: example-cc
  namespace: openfaas-fn
spec:
  community-size: 3
  maximum-delay: 100
  probability-threshold: 20
  iterations: 20
  slpa-service: slpa.default.svc.cluster.local:4567
status:
  generated-communities: []
```

* **Community Schedule**: `CommunitySchedule` defines the function placement and routing policies.

## Getting started

### Infrastructure Setup

NEPTUNE is a complex framework that requires a modified version of K3s for in-place vertical-autoscaling.

A Terraform repository to deploy a modified K3s cluster on the AWS Cloud can be found [here](https://github.com/deib-polimi/k3s-vertical-scaling-multi-regioncluster).

Otherwise it's necessary to setup a K3s distribution which integrate KEP 1287 ([link](https://github.com/kubernetes/enhancements/issues/1287)).

### Deploy a function

To deploy a function it's sufficient to deploy an OpenFaaS function Custom Resouce. An example is:
```
apiVersion: openfaas.com/v1
kind: Function
metadata:
  name: prime-numbers
  namespace: openfaas-fn
spec:
  image: systemautoscaler/prime-numbers:0.1.0
  labels:
    com.openfaas.scale.factor: "20"
    com.openfaas.scale.max: "100"
    com.openfaas.scale.min: "1"
    com.openfaas.scale.zero: "false"
    edgeautoscaler.polimi.it/scheduler: edge-autoscaler
  name: prime-numbers
  readOnlyRootFilesystem: false
  requests:
    memory: 1M
```
If you are using `Kubectl`, you can:
```
kubectl apply -f {function_configuration_file}.yaml
```

# CRDs code generation

Since the API code generator used in [hack/update-codegen.sh](hack/update-codegen.sh) was not designed to work with Go modules, it is mandatory to recreate the entire module path in order to make the code generation work.  
This gives you two options:  
1) Create the folders `github.com/deib-polimi` and clone this repository in any location of your filesystem.
2) Clone the repository inside the `GOPATH` directory.

In the end there is no choice other than to preserve the module hierarchy.

## Video Presentation
You can find a presentation of this work on [Youtube](https://youtu.be/bvtL6Ohme_4).

## Citation
If you use this code for evidential learning as part of your project or paper, please cite the following work:  

    @article{baresi2022neptune,
      title={{NEPTUNE}: Network- and GPU-aware Management of Serverless Functions at the Edge},
      author={Baresi, Luciano and Hu, Davide Yi Xian and Quattrocchi, Giovanni and Terracciano, Luca},
      journal={SEAMS},
      year={2022}
    }
    
## Contributors
* **[Davide Yi Xian Hu](https://github.com/DragonBanana)**
* **[Luca Terracciano](https://github.com/lterrac)**
