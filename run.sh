#!/bin/sh
# minikube delete
# minikube start
cd ~/code/mister
CGO_ENABLED=0 GOOS=linux go build -o bin/coordinator -a cmd/coordinator/coordinator.go
docker build . -t coordinator:1
minikube image load coordinator:1
cd ~/code/mister-apps/wordcount
CGO_ENABLED=0 GOOS=linux go build -o bin/mr-wordcount -a
docker build . -t wordcount:1
minikube image load wordcount:1
kubectl create clusterrolebinding default-view --clusterrole=admin --serviceaccount=default:default