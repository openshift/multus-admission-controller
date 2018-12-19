# Network attachment definition admission controller

An admission controller to check resources as defined by the NPWG spec upon their creation.

## Getting started

To quickly build and deploy admission controller run:
```
cd $GOPATH/src/github.com/K8sNetworkPlumbingWG/net-attach-def-admission-controller
make
kubectl apply -f deployments/rbac.yaml
kubectl apply -f deployments/install.yaml
kubectl apply -f deployments/server.yaml
```
For full installation and troubleshooting steps please see [Installation guide](docs/installation.md).

