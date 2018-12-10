# Installation guide

## Building Docker image
Go to the root directory of net-attach-def-admission-controller and execute:
```
cd $GOPATH/src/github.com/K8sNetworkPlumbingWG/net-attach-def-admission-controller
make
```

## Deploying webhook application
Create Service Account for net-attach-def-admission-controller webhook and webhook installer and apply RBAC rules to created account:
```
kubectl apply -f deployments/rbac.yaml
```

Next step runs Kubernetes Job which creates all resources required to run webhook:
* validating webhook configuration
* secret containing TLS key and certificate
* service to expose webhook deployment to the API server
Execute command:
```
kubectl apply -f deployments/install.yaml
```
*Note: Verify that Kubernetes controller manager has --cluster-signing-cert-file and --cluster-signing-key-file parameters set to paths to your CA keypair
to make sure that Certificates API is enabled in order to generate certificate signed by cluster CA.
More details about TLS certificates management in a cluster available [here](https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/).*

If Job has succesfully completed, you can run the actual webhook application.

Create webhook server Deployment:
```
kubectl apply -f deployments/server.yaml
```

## Verifying that validating webhook works
Try to create invalid Network Attachment Definition resource:
```
cat <<EOF | kubectl create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: invalid-net-attach-def
spec:
  config: '{
    "invalid": "config"
  }'
EOF
```
Webhook should deny the request:
```
Error from server: error when creating "STDIN": admission webhook "net-attach-def-admission-controller-validating-config.k8s.cni.cncf.io" denied the request: invalid config: error parsing configuration: missing 'type'
```

Now, try to create correctly defined one:
```
cat <<EOF | kubectl create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: correct-net-attach-def
spec:
  config: '{
    "cniVersion": "0.3.0",
    "name": "a-bridge-network",
    "type": "bridge",
    "bridge": "br0",
    "isGateway": true,
    "ipam": {
      "type": "host-local",
      "subnet": "192.168.5.0/24",
      "dataDir": "/mnt/cluster-ipam"
    }
  }'
EOF
```
Resource should be allowed and created:
```
networkattachmentdefinition.k8s.cni.cncf.io/correct-net-attach-def created
```

## Troubleshooting
Webhook server prints a lot of debug messages that could help to find the root cause of an issue.
To display logs run:
```
kubectl logs -l app=net-attach-def-admission-controller
```
Example output showing logs for handling requests generated in the "Verifying installation section":
```
2018/12/10 13:31:31 INFO: validating network config spec: { "invalid": "config" }
2018/12/10 13:31:31 INFO: spec is not a valid network config list: error parsing configuration list: no name - trying to parse into standalone config
2018/12/10 13:31:31 INFO: spec is not a valid network config: { "invalid": "config" }
2018/12/10 13:31:31 INFO: sending response to the Kubernetes API server
2018/12/10 13:31:36 INFO: validating network config spec: { "cniVersion": "0.3.0", "name": "a-bridge-network", "type": "bridge", "bridge": "br0", "isGateway": true, "ipam": { "type": "host-local", "subnet": "192.168.5.0/24", "dataDir": "/mnt/cluster-ipam" } }
2018/12/10 13:31:36 INFO: spec is not a valid network config list: error parsing configuration list: no 'plugins' key - trying to parse into standalone config
2018/12/10 13:31:36 INFO: AdmissionReview request allowed: Network Attachment Definition '{ "cniVersion": "0.3.0", "name": "a-bridge-network", "type": "bridge", "bridge": "br0", "isGateway": true, "ipam": { "type": "host-local", "subnet": "192.168.5.0/24", "dataDir": "/mnt/cluster-ipam" } }' is valid
2018/12/10 13:31:36 INFO: sending response to the Kubernetes API server
```

