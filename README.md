# NEPTUNE
<p align="center">
  <img width="100%" src="https://i.imgur.com/tm9mSuM.png" alt="Politecnico di Milano" />
</p>

## Overview

NEPTUNE is a comprehensive framework, developed at the Politecnico di Milano, for the runtime management of large-scale edge applications that exploits placement, routing, network delays, and CPU/GPU interplay in a coordinated way to allow for the concurrent execution of edge applications that meet user-set response times.

NEPTUNE implementation run on top of Kubernetes or Kubernetes-compatible orchestrator as a set of Kubernetes Controllers and Custom Resources.

## NEPTUNE Controllers

* **System Controller**: `SystemController` is a component that splits large edge topologies into smaller, closely-located and indenpendet communities.     Communities are found using an external function that, given a set of nodes and the latencies between all nodes, returns a set of communities and the set of nodes assigned to each community. 
  The base implementation of the `SystemController` adopts SLPA (Speaker-Listener Label Propagation Algorithm, Xie et al., 2019).
  The `SystemController` takes in input `CommunityConfigurations` Custom Resources and it splits the network of nodes according to the `CommunityCOnfigurations`.
  A network of nodes is split into communities by assigning to each node a label that denote to which community the node belongs to.

* **Community Controller**: .

* **Node Controller**: .

* **Request Dispatcher**: .

* **Monitoring**: .

* **Metrics Database**: .

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
