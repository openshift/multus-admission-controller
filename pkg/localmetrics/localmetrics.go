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
	"github.com/prometheus/client_golang/prometheus"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("network-attachment-definition")
var (
	netAttachDefInstanceEnabledCount      = 0.0
	netAttachDefInstanceSriovEnabledCount = 0.0
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
func UpdateNetAttachDefInstanceMetrics(tp string, val float64) {

	NetAttachDefInstanceCounter.With(prometheus.Labels{
		"networks": tp}).Add(val)

	if tp == "sriov" {
		netAttachDefInstanceSriovEnabledCount += val
		if netAttachDefInstanceSriovEnabledCount > 0.0 {
			SetNetAttachDefEnabledInstanceUp(tp, 1.0)
		} else {
			SetNetAttachDefEnabledInstanceUp(tp, 0.0)
		}
	} else if tp == "any" {
		netAttachDefInstanceEnabledCount += val
		if netAttachDefInstanceEnabledCount > 0.0 {
			SetNetAttachDefEnabledInstanceUp(tp, 1.0)
		} else {
			SetNetAttachDefEnabledInstanceUp(tp, 0.0)
		}
	}

}

//SetNetAttachDefEnabledInstanceUp ...
func SetNetAttachDefEnabledInstanceUp(tp string, val float64) {
	NetAttachDefEnabledInstanceUp.With(prometheus.Labels{
		"networks": tp}).Set(val)
}

//InitMetrics ... empty metrics
func InitMetrics() {
	UpdateNetAttachDefInstanceMetrics("any", 0.0)
	UpdateNetAttachDefInstanceMetrics("sriov", 0.0)
}
