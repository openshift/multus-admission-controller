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

package webhook

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
)

var _ = Describe("Webhook", func() {

	Describe("Preparing Admission Review Response", func() {
		Context("Admission Review Request is nil", func() {
			It("should return error", func() {
				ar := &admissionv1.AdmissionReview{}
				ar.Request = nil
				Expect(prepareAdmissionReviewResponse(false, "", ar)).To(HaveOccurred())
			})
		})
		Context("Message is not empty", func() {
			It("should set message in the response", func() {
				ar := &admissionv1.AdmissionReview{}
				ar.Request = &admissionv1.AdmissionRequest{
					UID: "fake-uid",
				}
				err := prepareAdmissionReviewResponse(false, "some message", ar)
				Expect(err).NotTo(HaveOccurred())
				Expect(ar.Response.Result.Message).To(Equal("some message"))
			})
		})
	})

	Describe("Deserializing Admission Review", func() {
		Context("It's not an Admission Review", func() {
			It("should return an error", func() {
				body := []byte("some-invalid-body")
				_, err := deserializeAdmissionReview(body)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Deserializing Network Attachment Definition", func() {
		Context("It's not an Network Attachment Definition", func() {
			It("should return an error", func() {
				ar := &admissionv1.AdmissionReview{}
				ar.Request = &admissionv1.AdmissionRequest{}
				_, err := deserializeNetworkAttachmentDefinition(ar)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Writing a response", func() {
		Context("with an AdmissionReview", func() {
			It("should be marshalled and written to a HTTP Response Writer", func() {
				w := httptest.NewRecorder()
				ar := &admissionv1.AdmissionReview{}
				ar.Response = &admissionv1.AdmissionResponse{
					UID:     "fake-uid",
					Allowed: true,
					Result: &metav1.Status{
						Message: "fake-msg",
					},
				}
				expected := []byte(`{"response":{"uid":"fake-uid","allowed":true,"status":{"metadata":{},"message":"fake-msg"}}}`)
				writeResponse(w, ar)
				Expect(w.Body.Bytes()).To(Equal(expected))
			})
		})
	})

	Describe("Handling requests", func() {
		Context("Request body is empty", func() {
			It("validate - should return an error", func() {
				req := httptest.NewRequest("POST", fmt.Sprintf("https://fakewebhook/validate"), nil)
				w := httptest.NewRecorder()
				ValidateHandler(w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("Content type is not application/json", func() {
			It("validate - should return an error", func() {
				req := httptest.NewRequest("POST", fmt.Sprintf("https://fakewebhook/validate"), bytes.NewBufferString("fake-body"))
				req.Header.Set("Content-Type", "invalid-type")
				w := httptest.NewRecorder()
				ValidateHandler(w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusUnsupportedMediaType))
			})
		})

		Context("Deserialization of net-attachment-def failed", func() {
			It("validate - should return an error", func() {
				req := httptest.NewRequest("POST", fmt.Sprintf("https://fakewebhook/validate"), bytes.NewBufferString("fake-body"))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				ValidateHandler(w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})

	DescribeTable("Network Attachment Definition validation",
		func(in netv1.NetworkAttachmentDefinition, out bool, shouldFail bool) {
			actualOut, err := validateNetworkAttachmentDefinition(in)
			Expect(actualOut).To(Equal(out))
			if shouldFail {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry(
			"empty config",
			netv1.NetworkAttachmentDefinition{},
			false, true,
		),
		Entry(
			"invalid name",
			netv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some?invalid?name",
				},
			},
			false, true,
		),
		Entry(
			"invalid network config",
			netv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-valid-name",
				},
				Spec: netv1.NetworkAttachmentDefinitionSpec{
					Config: `{"some-invalid": "config"}`,
				},
			},
			false, true,
		),
		Entry(
			"valid network config",
			netv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-valid-name",
				},
				Spec: netv1.NetworkAttachmentDefinitionSpec{
					Config: `{"cniVersion": "0.3.0", "type": "some-plugin"}`,
				},
			},
			true, false,
		),
		Entry(
			"valid network config list",
			netv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-valid-name",
				},
				Spec: netv1.NetworkAttachmentDefinitionSpec{
					Config: `{
						"cniVersion": "0.3.0",
						"name": "some-bridge-network",
						"plugins": [{
							"type": "bridge",
							"bridge": "br0",
							"ipam": {
								"type": "host-local",
								"subnet": "192.168.1.0/24"
							}
						},
						{
							"type": "some-plugin"
						},
						{
							"type": "another-plugin",
							"sysctl": {
								"net.ipv4.conf.all.log_martians": "1"
							}
						}]
					}`,
				},
			},
			true, false,
		),
		Entry(
			"validate missing name in config",
			netv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-valid-name",
				},
				Spec: netv1.NetworkAttachmentDefinitionSpec{
					Config: `{
						"cniVersion": "0.3.0",
						"plugins": [{
							"type": "bridge",
							"bridge": "br0",
							"ipam": {
								"type": "host-local",
								"subnet": "192.168.1.0/24"
							}
						},
						{
							"type": "some-plugin"
						},
						{
							"type": "another-plugin",
							"sysctl": {
								"net.ipv4.conf.all.log_martians": "1"
							}
						}]
					}`,
				},
			},
			true, false,
		),
	)
})
