// Copyright 2019 Network Plumbing Working Group
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package localmetrics

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricStoreInitSize int = 330
	initialMetricsCount int = 0
	metricsIncVal       int = 1
)

var (
	netAttachDefInstanceEnabledCount        = initialMetricsCount
	netAttachDefInstanceSriovEnabledCount   = initialMetricsCount
	netAttachDefInstanceIBSriovEnabledCount = initialMetricsCount
	//Change this when we set metrics per node.
	objStore = make(map[string]string, metricStoreInitSize) // Preallocate room 110 entires per node*3
	//NetAttachDefInstanceCounter ...  Total no of network attachment definition instance in the cluster
	NetAttachDefInstanceCounter = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_attachment_definition_instances",
			Help: "Metric to get number of instance using network attachment definition in the cluster.",
		}, []string{"networks"})
	//NetAttachDefEnabledInstanceUp  ... check if any instance with netattachdef config enabled
	NetAttachDefEnabledInstanceUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_attachment_definition_enabled_instance_up",
			Help: "Metric to identify clusters with network attachment definition enabled instances.",
		}, []string{"networks"})
)

//UpdateNetAttachDefInstanceMetrics ...
func UpdateNetAttachDefInstanceMetrics(tp string, val int) {

	glog.Infof("UPdating net-attach-def metrics for %s with value %d", tp, val)
	NetAttachDefInstanceCounter.With(prometheus.Labels{
		"networks": tp}).Add(float64(val))

	if tp == "sriov" {
		netAttachDefInstanceSriovEnabledCount += val
		if netAttachDefInstanceSriovEnabledCount > initialMetricsCount {
			SetNetAttachDefEnabledInstanceUp(tp, metricsIncVal)
		} else {
			SetNetAttachDefEnabledInstanceUp(tp, initialMetricsCount)
		}
	} else if tp == "ib-sriov" {
		netAttachDefInstanceIBSriovEnabledCount += val
		if netAttachDefInstanceIBSriovEnabledCount > initialMetricsCount {
			SetNetAttachDefEnabledInstanceUp(tp, metricsIncVal)
		} else {
			SetNetAttachDefEnabledInstanceUp(tp, initialMetricsCount)
		}
	} else if tp == "any" {
		netAttachDefInstanceEnabledCount += val
		if netAttachDefInstanceEnabledCount > initialMetricsCount {
			SetNetAttachDefEnabledInstanceUp(tp, metricsIncVal)
		} else {
			SetNetAttachDefEnabledInstanceUp(tp, initialMetricsCount)
		}
	}

}

//SetNetAttachDefEnabledInstanceUp ...
func SetNetAttachDefEnabledInstanceUp(tp string, val int) {
	NetAttachDefEnabledInstanceUp.With(prometheus.Labels{
		"networks": tp}).Set(float64(val))
}

//InitMetrics ... empty metrics
func InitMetrics() {
	UpdateNetAttachDefInstanceMetrics("any", initialMetricsCount)
	UpdateNetAttachDefInstanceMetrics("sriov", initialMetricsCount)
	UpdateNetAttachDefInstanceMetrics("ib-sriov", initialMetricsCount)
}

//GetStoredValue ... Get stroed config value for pod key
func GetStoredValue(key string) string {
	if value, ok := objStore[key]; ok {
		return value
	}
	return ""
}

//SetStoredValue // set stored key value
func SetStoredValue(key string, val string) {
	if val == "" {
		_, ok := objStore[key]
		if ok {
			delete(objStore, key)
		}
	} else {
		objStore[key] = val
	}
}
