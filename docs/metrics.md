---
Title: Metrics
---

Network attachment definition admission controller uses [Prometheus][prometheus] for metrics reporting. The metrics can be used for real-time monitoring and debugging. Network attachment definition admission controller does not persist its metrics; if a member restarts, the metrics will be reset.

The simplest way to see the available metrics is to cURL the metrics endpoint `/metrics`. The format is described [here](http://prometheus.io/docs/instrumenting/exposition_formats/).

Follow the [Prometheus getting started doc](http://prometheus.io/docs/introduction/getting_started/) to spin up a Prometheus server to collect Network attachment definition admission controller metrics.

The naming of metrics follows the suggested [Prometheus best practices](http://prometheus.io/docs/practices/naming/). 

A metric name has an `network_attachment_definition`  prefix as its namespace and a subsystem prefix .

## network_attachment_definition namespace metrics

The metrics under the `network_attachment_definition` prefix are for monitoring .  If there is any change of these metrics, it will be included in release notes.



### Metrics

These metrics describe the status of the network_attachment_definition resource and pod configured with this resource.

All these metrics are prefixed with `network_attachment_definition_`

| Name                                                  | Description                                              | Type    |
|-------------------------------------------------------|----------------------------------------------------------|---------|
| network_attachment_definition_instances          | Number of pods with k8s.v1.cni.cncf.io/networks configured.   | Gauge |
| network_attachment_definition_enabled_instance_up     | Whether or not a  k8s.v1.cni.cncf.io/networks annotated pods are running.  | Gauge   |
                                                        

`network_attachment_definition_instances` -  The number of pod with k8s.v1.cni.cncf.io/networks annotation  and types of networks configured via network attachment definition.  They are grouped by various network types.

Example 
``` 
network_attachment_definition_instances{networks="bridge"} 
//Total count for bridge types of network used by the pods.

network_attachment_definition_instances{networks="mcvlan,bridge"} 
//Total count for mcvlan and bridge types of network used by the pods.

network_attachment_definition_instances{networks="sriov"}  
// Total count for sriov types of network used by the pods.

network_attachment_definition_instances{networks="ib-sriov"}
// Total count for infiniband sriov types of network used by the pods.

network_attachment_definition_instances{networks="any"} 
// Total count for any types of network used by the pods.
```

`network_attachment_definition_enabled_instance_up` -  This metrics indicates whether the cluster has any pod running with network attachment definition configured. They are grouped by network types such as any, sriov only or ib-sriov only.

Example 
``` 
network_attachment_definition_enabled_instance_up{networks="sriov"} 
//Whether the cluster running an instance with  sriov type of network.

network_attachment_definition_enabled_instance_up{networks="ib-sriov"}
//Whether the cluster running an instance with  infiniband sriov type of network.

network_attachment_definition_enabled_instance_up{networks="any"} 
//Whether the cluster running an instance with  any type of network.

```
