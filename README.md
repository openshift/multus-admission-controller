# Network attachment definition admission controller

An admission controller to check resources as defined by the [Network Plumbing Working Group](https://github.com/K8sNetworkPlumbingWG/community) de-facto CRD specification upon their creation.

## Getting started

Clone this repository and apply these manifests:

```
kubectl apply -f deployments/rbac.yaml \
  -f deployments/install.yaml \
  -f deployments/server.yaml
```

## Building the admission controller

To build the admission controller, ensure it exists in your go path (we recommend you clone it to `$GOPATH/src/github.com/K8sNetworkPlumbingWG/net-attach-def-admission-controller`).

While in that directory, simple run:

```
make
```

For full installation and troubleshooting steps please see [Installation guide](docs/installation.md).

