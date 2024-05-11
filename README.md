# Webdav CSI driver for Kubernetes

### Overview

This is a repository for webdav csi driver, csi plugin name: `webdav.csi.io`. This driver supports dynamic provisioning of Persistent Volumes via Persistent Volume Claims by creating a new sub directory under webdav server.

### Quick start with kind 

#### Build plugin image
```bash
make docker-build
```

#### Start kind cluster
```bash
kind create cluster --image kindest/node:v1.27.3
```

### Load plugin image to kind cluster
```bash
kind load docker-image registry.k8s.io/sig-storage/csi-provisioner:v3.6.2
kind load docker-image registry.k8s.io/sig-storage/livenessprobe:v2.11.0
kind load docker-image registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.9.1
kind load docker-image localhost:5000/webdavplugin:v0.0.1
```

### Deploy CSI 
```bash
kubectl apply -f deploy/
```

### Tests
```bash
kubectl apply -f examples/csi-webdav-secret.yaml
kubectl apply -f examples/csi-webdav-storageclass.yaml
kubectl apply -f examples/csi-webdav-dynamic-pvc.yaml
kubectl apply -f examples/csi-webdav-pod.yaml
```