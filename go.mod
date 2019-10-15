module github.com/K8sNetworkPlumbingWG/net-attach-def-admission-controller

go 1.12

require (
	github.com/K8sNetworkPlumbingWG/network-attachment-definition-client v0.0.0-20191002070930-3de720f9c99b
	github.com/containernetworking/cni v0.7.0-alpha1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/intel/multus-cni v0.0.0-20180818113950-86af6ab69fe2
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.8.1
	k8s.io/api v0.0.0-20181115043458-b799cb063522
	k8s.io/apimachinery v0.0.0-20181110190943-2a7c93004028
	k8s.io/client-go v0.0.0-20181115111358-9bea17718df8
)
