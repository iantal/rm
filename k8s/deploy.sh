#!/bin/bash -e

kubectl apply -f rm-configmap.yml
kubectl apply -f rm-pv.yml
kubectl apply -f rm-pvc.yml
kubectl apply -f rm-service.yml
kubectl apply -f rm-statefulset.yml