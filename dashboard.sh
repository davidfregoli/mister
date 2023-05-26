./reset.sh
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/dashboard -a cmd/dashboard/dashboard.go
docker build -f dashboard.Dockerfile . -t dashboard:1
minikube image load dashboard:1
./apply.sh
./url.sh
