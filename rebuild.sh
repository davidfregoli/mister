# !/bin/sh
minikube delete
minikube start
cd ~/code/mister
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/coordinator -a cmd/coordinator/coordinator.go
docker build -f coordinator.Dockerfile . -t coordinator:1
minikube image load coordinator:1
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/dashboard -a cmd/dashboard/dashboard.go
docker build -f dashboard.Dockerfile . -t dashboard:1
minikube image load dashboard:1
cd ~/code/mister-wordcount
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/mr-wordcount -a
docker build . -t wordcount:1
minikube image load wordcount:1
kubectl create clusterrolebinding default-view --clusterrole=admin --serviceaccount=default:default