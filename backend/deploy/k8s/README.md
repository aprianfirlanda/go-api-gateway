# Kubernetes Skeleton

This directory contains Sprint 21 deployment skeleton manifests:

- `namespace.yaml`
- `configmap.yaml`
- `secret.example.yaml`
- `migrator-job.yaml`
- `gateway-deployment.yaml` + `gateway-service.yaml`
- `control-plane-deployment.yaml` + `control-plane-service.yaml`

Apply in order:

```sh
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/configmap.yaml
kubectl apply -f deploy/k8s/secret.example.yaml
kubectl apply -f deploy/k8s/migrator-job.yaml
kubectl apply -f deploy/k8s/control-plane-deployment.yaml -f deploy/k8s/control-plane-service.yaml
kubectl apply -f deploy/k8s/gateway-deployment.yaml -f deploy/k8s/gateway-service.yaml
```

Replace image names (`syra/*:latest`) and secret values before production use.
