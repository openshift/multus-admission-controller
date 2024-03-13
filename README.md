XXX
# Network attachment definition admission controller

An admission controller to check resources as defined by the [Network Plumbing Working Group](https://github.com/k8snetworkplumbingwg/community) de-facto CRD specification upon their creation.

This admission controller is aware of some of the aspects of what's required when you create `NetworkAttachmentDefinition` custom resources, and can report back to the user that those resources are well formatted, to improve their experience.

## Getting started

Clone this repository and execute `./hack/webhook-deployment.sh` to deploy:

```
$ ./hack/webhook-deployment.sh
```

## Example of the admission controller in action

If you're to create a `NetworkAttachmentDefinition` which has something that's out of whack -- in this example malformatted JSON -- you'll get some feedback that something is wrong.

```
$ cat <<EOF | kubectl create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: bad-conf
spec:
  config: '{
      "cniVersion": "0.3.0"malFormattedJSON
      }
    }'
EOF
Error from server: error when creating "STDIN": admission webhook "net-attach-def-admission-controller-validating-config.k8s.cni.cncf.io" denied the request: invalid config: error parsing configuration: invalid character 'm' after object key:value pair
```

If you create a valid `NetworkAttachmentDefinition`, you'll find that the custom resource is created successfully.

```
$ cat <<EOF | kubectl create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-conf
spec:
  config: '{
      "cniVersion": "0.3.0",
      "type": "macvlan",
      "master": "eth0",
      "mode": "bridge",
      "ipam": {
        "type": "host-local",
        "subnet": "192.168.1.0/24",
        "rangeStart": "192.168.1.200",
        "rangeEnd": "192.168.1.216",
        "routes": [
          { "dst": "0.0.0.0/0" }
        ],
        "gateway": "192.168.1.1"
      }
    }'
EOF
networkattachmentdefinition.k8s.cni.cncf.io/macvlan-conf created
```

## Collecting metrics with Prometheus
Network attachment definition admission controller comes with following metrics.
  1. No. of instances with k8s.v1.cni.cncf.io/networks annotations 
  2. Is cluster enabled with instances annotated with k8s.v1.cni.cncf.io/networks.

To install Prometheus and enable scraping the endpoints , execute `./hack/prometheus-deployment.sh` 

[Metrics details ](docs/metrics.md)

## Building the admission controller

To build the admission controller, ensure it exists in your go path (we recommend you clone it to `$GOPATH/src/github.com/k8snetworkplumbingwg/net-attach-def-admission-controller`).

While in that directory, simple run:

```
make
```

For full installation and troubleshooting steps please see [Installation guide](docs/installation.md).

For developer information, refer to the [developer guide](docs/developer.md).

