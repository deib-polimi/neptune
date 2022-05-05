# NEPTUNE
<p align="center">
  <img width="100%" src="https://i.imgur.com/tm9mSuM.png" alt="Politecnico di Milano" />
</p>

## Overview

NEPTUNE is a comprehensive framework, developed at the Politecnico di Milano, for the runtime management of large-scale edge applications that exploits placement, routing, network delays, and CPU/GPU interplay in a coordinated way to allow for the concurrent execution of edge applications that meet user-set response times.

NEPTUNE implementation run on top of Kubernetes or Kubernetes-compatible orchestrator as a set of Kubernetes Controllers and Custom Resources.

## NEPTUNE Controllers

* **System Controller**: `SystemController` is the component that splits large edge topologies into smaller, closely-located and indenpendet communities.     Communities are found using an external function that, given a set of nodes and the latencies between all nodes, returns a set of communities and the set of nodes assigned to each community. 
    The base implementation of the `SystemController` adopts SLPA (Speaker-Listener Label Propagation Algorithm, Xie et al., 2019). The implementation of the SLPA algorithm can be found [here](link)
    The `SystemController` takes in input `CommunityConfigurations` Custom Resources and it splits the network of nodes according to `CommunityConfigurations`.
    A network of nodes is split into communities by assigning to each node a label that denote to which community the node belongs to.

* **Community Controller**: `CommunityController` is the component that schedules functions on nodes and provides routing policies for smart requests routing.
    As in the `SystemController`, function schedules and routing policies are found using an external function that requires the information about the topology and the set of functions to be deployed on top of the nodes. 
    The `CommunityController` relies on solving a two-step mixed integer linear programming problem. The solver can be found [here](link)
    The `CommunityController` is composed by two control loops: the first one updates `CommunitySchedule` using the two-step mixed integer linear programming, and the second one tries to reconcile the actual function schedule and the one specified in `CommunitySchedule`.

* **Node Controller**: `NodeController` is the component that vertical scales function in order to meet user-set response times. The node controller has been implemented in another project named Kosmos. The project can be found [here](link)

* **Request Dispatcher**: `RequestDispacher` is the component that routes requests to function instances according to the routing policies specified in `CommunitySchedule`. The requests dispatcher works only with HTTP and HTTPS messages.

* **Monitoring**: `Monitoring` is the component that monitors and collect informations about resource consumption and network performance inside the cluster.

* **Metrics Database**: `MetricsDatabase` is a SQL database that stores data. It has been chosen to use TimeScaleDB, but since it has an interface PostGresQL-compatible, it's also possible to use PSQL. 

## NEPTUNE Resources

* **Community Setting**: .

* **Community Schedule**: .

## How to run

Serve anche la repo Java
e la repo python
Modificare i binari di k3s
Kosmos
Terraform

## Esempio di come lanciare sta merda

Come lanciare una funzione
Guardare example

## Video Presentation
You can find a presentation of this work on [Youtube](https://link.com).

## Citation
If you use this code for evidential learning as part of your project or paper, please cite the following work:  

    @article{baresi2020neptune,
      title={NEPTUNE: Network- and GPU-aware Management of Serverless Functions at the Edge},
      author={Baresi, Luciano and Hu, Davide Yi Xian and Quattrocchi, Giovanni and Terracciano, Luca},
      journal={SEAMS},
      volume={33},
      year={2022}
    }
    
## Contributors
* **[Davide Yi Xian Hu](https://github.com/DragonBanana)**
* **[Luca Terracciano](https://github.com/lterrac)**
