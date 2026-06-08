// Copyright (c) 2026 Network Plumbing Working Group
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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var (
	_ = Describe("StringSliceFlag", testStringSliceFlag)
	_ = Describe("HTTP Servers", testHTTPServers)
)

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

func testStringSliceFlag() {
	Context("String", func() {
		Specify("should return empty string when nil", func() {
			var flag *StringSliceFlag
			Expect(flag.String()).To(Equal(""))
		})

		Specify("should return comma-separated string", func() {
			flag := &StringSliceFlag{"one", "two", "three"}
			Expect(flag.String()).To(Equal(
				"one,two,three"))
		})
	})

	DescribeTable("Set",
		func(input string, expected []string) {
			var flag StringSliceFlag
			Expect(flag.Set(input)).To(Succeed())
			Expect(flag).To(Equal(StringSliceFlag(expected)))
		},
		Entry("with single value", "one", []string{"one"}),
		Entry("with multiple values", " one,two,three", []string{"one", "two", "three"}),
		Entry("with values with spaces", " one, two ,three ", []string{"one", "two", "three"}),
		Entry("with empty string", "", []string{}),
		Entry("with empty values", "one,,two,", []string{"one", "two"}),
	)
}

func testHTTPServers() {
	var (
		certFile string
		keyFile  string
		config   *ServerConfig
		cleanup  func()
	)

	BeforeEach(func() {
		// Generate test certificate and key
		var err error
		certFile, keyFile, err = generateTestCertificate()
		Expect(err).NotTo(HaveOccurred())

		config = &ServerConfig{
			Address:        "127.0.0.1",
			MetricsAddress: net.JoinHostPort("127.0.0.5", strconv.Itoa(getFreePort("127.0.0.5"))),
		}

		config.Port = getFreePort(config.Address)

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).NotTo(HaveOccurred())

		config.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &cert, nil
		}
	})

	AfterEach(func() {
		if cleanup != nil {
			cleanup()
		}
		if certFile != "" {
			_ = os.Remove(certFile)
		}
		if keyFile != "" {
			_ = os.Remove(keyFile)
		}
	})

	Context("with invalid TLS cipher suite", func() {
		It("should return error", func() {
			config.TLSCipherSuites = StringSliceFlag{"INVALID_CIPHER"}
			_, err := startHTTPServers(config)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with invalid TLS min version", func() {
		It("should return error", func() {
			config.TLSMinVersion = "InvalidVersion"
			_, err := startHTTPServers(config)
			Expect(err).To(HaveOccurred())
		})
	})

	DescribeTable("should reject TLS min versions below TLS 1.2",
		func(version string) {
			config.TLSMinVersion = version
			_, err := startHTTPServers(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("below the minimum required version TLS 1.2"))
		},
		Entry("TLS 1.0", "VersionTLS10"),
		Entry("TLS 1.1", "VersionTLS11"),
	)

	DescribeTable("should accept TLS min versions at or above TLS 1.2",
		func(version string) {
			config.TLSMinVersion = version
			var err error
			cleanup, err = startHTTPServers(config)
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("TLS 1.2", "VersionTLS12"),
		Entry("TLS 1.3", "VersionTLS13"),
	)

	Context("Webhook", func() {
		var bindAddress string

		BeforeEach(func() {
			bindAddress = net.JoinHostPort(config.Address, strconv.Itoa(config.Port))
		})

		JustBeforeEach(func() {
			var err error
			cleanup, err = startHTTPServers(config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should serve endpoints over HTTPS", func() {
			for _, endpoint := range []string{"/validate", "/isolate"} {
				testHTTPSEndpoint(bindAddress, endpoint)
			}
		})

		Context("with no TLS config options specified", func() {
			It("should use the default min version", func() {
				testTLSHandshake(bindAddress, tls.VersionTLS12, nil, func(g Gomega, state tls.ConnectionState, err error) {
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(int(state.Version)).To(Equal(tls.VersionTLS12))
					g.Expect(state.CipherSuite).To(Or(
						Equal(tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256),
						Equal(tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384),
						Equal(tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256)))
				})
			})
		})

		Context("with TLS ciphers specified", func() {
			BeforeEach(func() {
				config.TLSMinVersion = "VersionTLS12"

				// Configure server to explicitly NOT accept ChaCha20-Poly1305.
				// Only accept AES-based cipher suites
				config.TLSCipherSuites = StringSliceFlag{
					"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
					"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				}
			})

			It("should enforce the custom cipher suite restrictions", func() {
				// Client offers ONLY ChaCha20-Poly1305, which server explicitly excludes so handshake should fail.
				testTLSHandshake(bindAddress, tls.VersionTLS12, []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				}, func(g Gomega, _ tls.ConnectionState, err error) {
					g.Expect(err).To(HaveOccurred())
					g.Expect(err.Error()).To(ContainSubstring("handshake failure"))
				})
			})
		})

		Context("with the TLS minimum version specified", func() {
			BeforeEach(func() {
				config.TLSMinVersion = "VersionTLS13"
			})

			It("should reject versions below the minimum", func() {
				testShouldRejectTLSVersions(bindAddress)
			})
		})
	})

	Context("Metrics", func() {
		JustBeforeEach(func() {
			var err error
			cleanup, err = startHTTPServers(config)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with encryption enabled", func() {
			BeforeEach(func() {
				config.EncryptMetrics = true
			})

			Context("and the TLS minimum version specified", func() {
				BeforeEach(func() {
					config.TLSMinVersion = "VersionTLS13"
				})

				It("should reject versions below the minimum", func() {
					testShouldRejectTLSVersions(config.MetricsAddress)
				})
			})

			It("should serve endpoints over HTTPS", func() {
				for _, endpoint := range []string{"/metrics", "/healthz"} {
					testHTTPSEndpoint(config.MetricsAddress, endpoint)
				}
			})
		})

		Context("with encryption disabled", func() {
			It("should serve endpoints over plain HTTP", func() {
				for _, endpoint := range []string{"/metrics", "/healthz"} {
					testHTTPEndpoint(config.MetricsAddress, endpoint)
				}
			})
		})
	})
}

func testHTTPEndpoint(address, endpoint string) {
	Eventually(func(g Gomega) {
		resp, err := http.Get(fmt.Sprintf("http://%s%s", address, endpoint))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp.StatusCode).NotTo(Equal(http.StatusNotFound))
		resp.Body.Close()
	}).Within(5 * time.Second).Should(Succeed())
}

func testHTTPSEndpoint(address, endpoint string) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	Eventually(func(g Gomega) {
		resp, err := client.Get(fmt.Sprintf("https://%s%s", address, endpoint))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp.StatusCode).NotTo(Equal(http.StatusNotFound))
		resp.Body.Close()
	}).Within(5 * time.Second).Should(Succeed())
}

func getFreePort(addr string) int {
	listener, err := net.Listen("tcp", addr+":0")
	Expect(err).NotTo(HaveOccurred())

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	return port
}

func testShouldRejectTLSVersions(bindAddress string) {
	for clientVersion := tls.VersionTLS11; clientVersion <= tls.VersionTLS12; clientVersion++ {
		testTLSHandshake(bindAddress, clientVersion, nil, func(g Gomega, _ tls.ConnectionState, err error) {
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(Or(
				ContainSubstring("protocol version"),
				ContainSubstring("handshake failure"),
				ContainSubstring("tls: no supported versions"),
			))
		})
	}
}

func testTLSHandshake(bindAddress string, clientVersion int, clientCipherSuites []uint16, verify func(Gomega, tls.ConnectionState, error)) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         uint16(clientVersion),
		MaxVersion:         uint16(clientVersion),
		CipherSuites:       clientCipherSuites,
	}

	Eventually(func(g Gomega) {
		conn, err := tls.Dial("tcp", bindAddress, tlsConfig)
		if err != nil {
			verify(g, tls.ConnectionState{}, err)
			return
		}
		defer conn.Close()

		state := conn.ConnectionState()
		g.Expect(state.HandshakeComplete).To(BeTrue())
		verify(g, state, err)
	}).Within(5 * time.Second).Should(Succeed())
}

// generateTestCertificate creates a self-signed ECDSA certificate for testing
func generateTestCertificate() (string, string, error) {
	// Generate ECDSA private key (required for ECDSA cipher suites)
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.5")},
	}

	// Create certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", err
	}

	// Write certificate to temp file
	certFile, err := os.CreateTemp("", "test-cert-*.pem")
	if err != nil {
		return "", "", err
	}
	defer certFile.Close()

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		_ = os.Remove(certFile.Name())
		return "", "", err
	}

	// Write private key to temp file
	keyFile, err := os.CreateTemp("", "test-key-*.pem")
	if err != nil {
		_ = os.Remove(certFile.Name())
		return "", "", err
	}
	defer keyFile.Close()

	// Marshal ECDSA private key
	privKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		_ = os.Remove(certFile.Name())
		_ = os.Remove(keyFile.Name())
		return "", "", err
	}

	err = pem.Encode(keyFile, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privKeyBytes,
	})
	if err != nil {
		_ = os.Remove(certFile.Name())
		_ = os.Remove(keyFile.Name())
		return "", "", err
	}

	return certFile.Name(), keyFile.Name(), nil
}
