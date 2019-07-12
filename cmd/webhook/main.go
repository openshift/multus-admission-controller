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

	"github.com/K8sNetworkPlumbingWG/net-attach-def-admission-controller/pkg/webhook"
	"github.com/golang/glog"
)

func main() {
	/* load configuration */
	port := flag.Int("port", 443, "The port on which to serve.")
	address := flag.String("bind-address", "0.0.0.0", "The IP address on which to listen for the --port port.")
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

	/* init API client */
	webhook.SetupInClusterClient()

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

		err := httpServer.ListenAndServeTLS(*cert, *key)
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
