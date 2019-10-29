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

var log = logf.Log.WithName("netdefattachment")
var (
	netDefInstanceEnabledCount      = 0.0
	netDefInstanceSriovEnabledCount = 0.0
	//NetDefAttachInstanceCounter ...  Total no of network attachment definition instance in the cluster
	NetDefAttachInstanceCounter = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_attachment_definition_instance_total",
			Help: "Metric to get total instance using network attachment definition.",
		}, []string{"networks"})
	//NetDefAttachEnabledInstanceUp  ... check if any instance with netattachdef config enabled
	NetDefAttachEnabledInstanceUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_attachment_definition_enabled_instance_up",
			Help: "Metric to identify clusters with network attachment definition enabled instances.",
		}, []string{"networks"})
)

//UpdateNetDefAttachInstanceMetrics ...
func UpdateNetDefAttachInstanceMetrics(tp string, val float64) {

	NetDefAttachInstanceCounter.With(prometheus.Labels{
		"networks": tp}).Add(val)

	if tp == "sriov" {
		netDefInstanceSriovEnabledCount += val
		if netDefInstanceSriovEnabledCount > 0.0 {
			SetNetDefAttachEnabledInstanceUp(tp, 1.0)
		} else {
			SetNetDefAttachEnabledInstanceUp(tp, 0.0)
		}
	} else if tp == "any" {
		netDefInstanceEnabledCount += val
		if netDefInstanceEnabledCount > 0.0 {
			SetNetDefAttachEnabledInstanceUp(tp, 1.0)
		} else {
			SetNetDefAttachEnabledInstanceUp(tp, 0.0)
		}
	}

}

//SetNetDefAttachEnabledInstanceUp ...
func SetNetDefAttachEnabledInstanceUp(tp string, val float64) {
	NetDefAttachEnabledInstanceUp.With(prometheus.Labels{
		"networks": tp}).Set(val)
}
