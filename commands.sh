kubectl create deployment coordinator --image=coordinator:1 --replicas=1
kubectl expose deployment coordinator --type=NodePort --name=coordinator --port=1234
kubectl exec --stdin --tty ~/code/mister/deployments/coordinator -- /bin/sh
kubectl apply -f ~/code/mister/deployments/coordinator-deployment.yaml
