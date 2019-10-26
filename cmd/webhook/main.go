// Copyright (c) 2018 Intel Corporation
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

package main

import (
	"crypto/sha512"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/k8snetworkplumbingwg/net-attach-def-admission-controller/pkg/controller"
	"github.com/k8snetworkplumbingwg/net-attach-def-admission-controller/pkg/localmetrics"
	"github.com/k8snetworkplumbingwg/net-attach-def-admission-controller/pkg/webhook"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	metricsPath = "/metrics"
	healthzPath = "/healthz"
)

func main() {
	/* load configuration */
	port := flag.Int("port", 443, "The port on which to serve.")
	address := flag.String("bind-address", "0.0.0.0", "The IP address on which to listen for the --port port.")
	metricsAddress := flag.String("metrics-listen-address", ":9091", "metrics server listen address.")
	cert := flag.String("tls-cert-file", "cert.pem", "File containing the default x509 Certificate for HTTPS.")
	key := flag.String("tls-private-key-file", "key.pem", "File containing the default x509 private key matching --tls-cert-file.")
	flag.Parse()

	glog.Infof("starting net-attach-def-admission-controller webhook server")

	keyPair, err := webhook.NewTlsKeypairReloader(*cert, *key)
	if err != nil {
		glog.Fatalf("error load certificate: %s", err.Error())
	}

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		glog.Fatalf("error to get process info: %s", err.Error())
	}

	// Register metrics
	prometheus.MustRegister(localmetrics.NetDefAttachInstanceCounter)
	prometheus.MustRegister(localmetrics.NetDefAttachEnabledInstanceUp)

	// Including these stats kills performance when Prometheus polls with multiple targets
	prometheus.Unregister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	prometheus.Unregister(prometheus.NewGoCollector())

	/* init API client */
	webhook.SetupInClusterClient()
	// start metrics sever
	startHTTPMetricServer(*metricsAddress)

	//Start watching for pod creations
	go controller.StartWatching()

	go func() {
		/* register handlers */
		var httpServer *http.Server
		http.HandleFunc("/validate", webhook.ValidateHandler)

		http.HandleFunc("/isolate", webhook.IsolateHandler)

		/* start serving */
		httpServer = &http.Server{
			Addr: fmt.Sprintf("%s:%d", *address, *port),
			TLSConfig: &tls.Config{
				GetCertificate: keyPair.GetCertificateFunc(),
			},
		}

		err := httpServer.ListenAndServeTLS("", "")
		if err != nil {
			glog.Fatalf("error starting web server: %v", err)
		}
	}()

	/* watch the cert file and restart http sever if the file updated. */
	oldHashVal := ""
	for {
		hasher := sha512.New()
		s, err := ioutil.ReadFile(*cert)
		hasher.Write(s)
		if err != nil {
			glog.Fatalf("failed to read file %s: %v", *cert, err)
		}
		newHashVal := hex.EncodeToString(hasher.Sum(nil))
		if oldHashVal != "" && newHashVal != oldHashVal {
			if err := proc.Signal(syscall.SIGHUP); err != nil {
				glog.Fatalf("failed to send certificate update notification: %v", err)
			}
		}
		oldHashVal = newHashVal

		time.Sleep(1 * time.Second)
	}

}

func startHTTPMetricServer(metricsAddress string) {
	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.Handler())

	// Add healthzPath
	mux.HandleFunc(healthzPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})
	// Add index
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		 <head><title>Net Attach Definition Admission Controller Metrics Server</title></head>
		 <body>
		 <h1>Kube Metrics</h1>
		 <ul>
		 <li><a href='` + metricsPath + `'>metrics</a></li>
		 <li><a href='` + healthzPath + `'>healthz</a></li>
		 </ul>
		 </body>
		 </html>`))
	})

	go utilwait.Until(func() {
		err := http.ListenAndServe(metricsAddress, mux)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("starting metrics server failed: %v", err))
		}
	}, 5*time.Second, utilwait.NeverStop)

}
