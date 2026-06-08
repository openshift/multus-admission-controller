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

// This is admission controller for network-attachment-definition.
package main

import (
	"context"
	"crypto/sha512"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/k8snetworkplumbingwg/net-attach-def-admission-controller/pkg/controller"
	"github.com/k8snetworkplumbingwg/net-attach-def-admission-controller/pkg/localmetrics"
	"github.com/k8snetworkplumbingwg/net-attach-def-admission-controller/pkg/webhook"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	cliflag "k8s.io/component-base/cli/flag"
)

const (
	metricsPath = "/metrics"
	healthzPath = "/healthz"
)

// ServerConfig holds configuration for the HTTP servers
type ServerConfig struct {
	Port            int
	Address         string
	MetricsAddress  string
	EncryptMetrics  bool
	TLSMinVersion   string
	TLSCipherSuites StringSliceFlag
	GetCertificate  func(*tls.ClientHelloInfo) (*tls.Certificate, error)
}

// StringSliceFlag implements flag.Value interface for comma-separated string lists
type StringSliceFlag []string

func (s *StringSliceFlag) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}

func (s *StringSliceFlag) Set(value string) error {
	parts := strings.Split(value, ",")
	*s = make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			*s = append(*s, trimmed)
		}
	}
	return nil
}

func main() {
	// load configuration
	cert := flag.String("tls-cert-file", "cert.pem", "File containing the default x509 Certificate for HTTPS.")
	key := flag.String("tls-private-key-file", "key.pem", "File containing the default x509 private key matching --tls-cert-file.")

	config := &ServerConfig{}
	flag.IntVar(&config.Port, "port", 443, "The port on which to serve.")
	flag.StringVar(&config.Address, "bind-address", "0.0.0.0", "The IP address on which to listen for the --port port.")
	flag.StringVar(&config.MetricsAddress, "metrics-listen-address", ":9091", "metrics server listen address.")
	flag.BoolVar(&config.EncryptMetrics, "encrypt-metrics", false, "serve metrics over HTTPS using tls-cert-file/tls-private-key-file x509 key pair")
	flag.StringVar(&config.TLSMinVersion, "tls-min-version", "", "Minimum TLS version supported")
	flag.Var(&config.TLSCipherSuites, "tls-cipher-suites", "Comma-separated list of cipher suites")

	var ignoreNamespaces StringSliceFlag
	flag.Var(&ignoreNamespaces, "ignore-namespaces", "Comma separated namespace list to ignore pod update")

	flag.Parse()

	glog.Infof("starting net-attach-def-admission-controller webhook server")

	keyPair, err := webhook.NewTLSKeypairReloader(*cert, *key)
	if err != nil {
		glog.Fatalf("error load certificate: %s", err.Error())
	}
	config.GetCertificate = keyPair.GetCertificateFunc()

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		glog.Fatalf("error to get process info: %s", err.Error())
	}

	// Register metrics
	prometheus.MustRegister(localmetrics.NetAttachDefInstanceCounter)
	prometheus.MustRegister(localmetrics.NetAttachDefEnabledInstanceUp)

	// Including these stats kills performance when Prometheus polls with multiple targets
	prometheus.Unregister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	prometheus.Unregister(prometheus.NewGoCollector())

	// init API client
	webhook.SetupInClusterClient()

	// Start HTTP servers (metrics and webhook)
	cleanup, err := startHTTPServers(config)
	if err != nil {
		glog.Fatalf("error starting HTTP servers: %v", err)
	}
	defer cleanup()

	// Start watching for pod creations
	go controller.StartWatching(ignoreNamespaces)

	// watch the cert file and restart http sever if the file updated.
	oldHashVal := ""
	for {
		hasher := sha512.New()
		certPath, err := filepath.Abs(*cert)
		if err != nil {
			glog.Fatalf("illegal path %s in certPath: %s: %v", *cert, certPath, err)
			os.Exit(1)
		}
		s, err := ioutil.ReadFile(certPath)
		hasher.Write(s)
		if err != nil {
			glog.Fatalf("failed to read file %s: %v", *cert, err)
			os.Exit(1)
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

func startHTTPServers(config *ServerConfig) (func(), error) {
	// Parse TLS configuration
	tlsCipherSuiteIDs, err := cliflag.TLSCipherSuites(config.TLSCipherSuites)
	if err != nil {
		return nil, fmt.Errorf("error parsing TLS cipher suites %v: %w", config.TLSCipherSuites, err)
	}

	var tlsMinVersionID uint16
	if config.TLSMinVersion != "" {
		tlsMinVersionID, err = cliflag.TLSVersion(config.TLSMinVersion)
		if err != nil {
			return nil, fmt.Errorf("error parsing TLS min version %q: %w", config.TLSMinVersion, err)
		}

		// Validate that the minimum TLS version is at least TLS 1.2
		if tlsMinVersionID < tls.VersionTLS12 {
			return nil, fmt.Errorf("TLS min version %q is below the minimum required version TLS 1.2", config.TLSMinVersion)
		}
	}

	applyTLSOptions := func(to *tls.Config) *tls.Config {
		if tlsMinVersionID != 0 {
			to.MinVersion = tlsMinVersionID
		}

		if len(tlsCipherSuiteIDs) > 0 {
			to.CipherSuites = tlsCipherSuiteIDs
		}

		to.GetCertificate = config.GetCertificate

		return to
	}

	// Start metrics server
	var metricsServer *http.Server
	if config.EncryptMetrics {
		metricsServer = startHTTPMetricServer(config.MetricsAddress, applyTLSOptions(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}))
	} else {
		metricsServer = startHTTPMetricServer(config.MetricsAddress, nil)
	}

	// Start webhook server
	webhookServer := &http.Server{
		Addr: fmt.Sprintf("%s:%d", config.Address, config.Port),
		TLSConfig: applyTLSOptions(&tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			},
		}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", webhook.ValidateHandler)
	mux.HandleFunc("/isolate", webhook.IsolateHandler)
	webhookServer.Handler = mux

	go func() {
		err := webhookServer.ListenAndServeTLS("", "")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			glog.Fatalf("error starting web server: %v", err)
		}
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := webhookServer.Shutdown(ctx); err != nil {
			glog.Errorf("error shutting down webhook server: %v", err)
		}
		if err := metricsServer.Shutdown(ctx); err != nil {
			glog.Errorf("error shutting down metrics server: %v", err)
		}
	}, nil
}

func startHTTPMetricServer(metricsAddress string, tlsConfig *tls.Config) *http.Server {
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

	srv := &http.Server{
		Addr:      metricsAddress,
		TLSConfig: tlsConfig,
		Handler:   mux,
	}

	go func() {
		var err error
		if tlsConfig != nil {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			glog.Fatalf("error starting metrics server: %v", err)
		}
	}()

	return srv
}
