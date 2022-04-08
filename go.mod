module github.com/k8snetworkplumbingwg/net-attach-def-admission-controller

go 1.12

require (
	github.com/containernetworking/cni v0.7.0-alpha1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/intel/multus-cni v0.0.0-20180818113950-86af6ab69fe2
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20191025120722-4c57cd5732f3
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.1
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go v0.0.0-20190918200256-06eb1244587a
	k8s.io/utils v0.0.0-20190506122338-8fab8cb257d5 // indirect
)

replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
