#!/bin/bash

kubectl create ns openfaas
kubectl create ns openfaas-fn

helm repo add openfaas https://openfaas.github.io/faas-netes/
helm install openfaas-k3s openfaas/openfaas  --namespace openfaas --set basic_auth=false --set functionNamespace=openfaas-fn --set operator.create=true --wait

kubectl apply -f config/crd/bases
kubectl apply -f config/permissions
kubectl apply -f config/deploy/allocation-algorithm.yaml
kubectl apply -f config/deploy/slpa.yaml
kubectl apply -f config/deploy/timescale-db.yaml
kubectl apply -f config/deploy/system-controller.yaml

sleep 15

kubectl apply -f config/deploy/dispatcher.yaml
kubectl apply -f config/deploy/prime-numbers-function.yaml
kubectl apply -f config/deploy/example-cs.yaml
