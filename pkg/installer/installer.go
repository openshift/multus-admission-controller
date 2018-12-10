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

package installer

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/csr"

	arv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	clientset kubernetes.Interface
	namespace string
	prefix    string
)

func generateCSR() ([]byte, []byte, error) {
	log.Printf("INFO: generating Certificate Signing Request")
	serviceName := strings.Join([]string{prefix, "service"}, "-")
	certRequest := csr.New()
	certRequest.KeyRequest = &csr.BasicKeyRequest{"rsa", 2048}
	certRequest.CN = strings.Join([]string{serviceName, namespace, "svc"}, ".")
	certRequest.Hosts = []string{
		serviceName,
		strings.Join([]string{serviceName, namespace}, "."),
		strings.Join([]string{serviceName, namespace, "svc"}, "."),
	}
	return csr.ParseRequest(certRequest)
}

func getSignedCertificate(request []byte) ([]byte, error) {
	csrName := strings.Join([]string{prefix, "csr"}, "-")
	csr, err := clientset.CertificatesV1beta1().CertificateSigningRequests().Get(csrName, metav1.GetOptions{})
	if csr != nil && err == nil {
		log.Printf("INFO: CSR %s already exists, trying to reuse it", csrName)
	} else {
		log.Printf("INFO: creating CSR %s", csrName)
		/* build Kubernetes CSR object */
		csr := &v1beta1.CertificateSigningRequest{}
		csr.ObjectMeta.Name = csrName
		csr.ObjectMeta.Namespace = namespace
		csr.Spec.Request = request
		csr.Spec.Groups = []string{"system:authenticated"}
		csr.Spec.Usages = []v1beta1.KeyUsage{v1beta1.UsageDigitalSignature, v1beta1.UsageServerAuth, v1beta1.UsageKeyEncipherment}

		/* push CSR to Kubernetes API server */
		csr, err = clientset.CertificatesV1beta1().CertificateSigningRequests().Create(csr)
		if err != nil {
			return nil, fmt.Errorf("error creating CSR in Kubernetes API: %s", err)
		}
		log.Printf("INFO: CSR pushed to the Kubernetes API")
	}

	if csr.Status.Certificate != nil {
		log.Printf("INFO: using already issued certificate for CSR %s", csrName)
		return csr.Status.Certificate, nil
	}
	/* approve certificate in K8s API */
	csr.ObjectMeta.Name = csrName
	csr.ObjectMeta.Namespace = namespace
	csr.Status.Conditions = append(csr.Status.Conditions, v1beta1.CertificateSigningRequestCondition{
		Type:           v1beta1.CertificateApproved,
		Reason:         "Approved by net-attach-def admission controller installer",
		Message:        "This CSR was approved by net-attach-def admission controller installer.",
		LastUpdateTime: metav1.Now(),
	})
	csr, err = clientset.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(csr)
	log.Printf("INFO: certificate approval sent")
	if err != nil {
		return nil, fmt.Errorf("error approving CSR in Kubernetes API: %s", err)
	}

	/* wait for the cert to be issued */
	log.Printf("INFO: waiting for the signed certificate to be issued...")
	start := time.Now()
	for range time.Tick(time.Second) {
		csr, err = clientset.CertificatesV1beta1().CertificateSigningRequests().Get(csrName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting signed ceritificate from the API server: %s", err)
		}
		if csr.Status.Certificate != nil {
			return csr.Status.Certificate, nil
		}
		if time.Since(start) > 60*time.Second {
			break
		}
	}

	return nil, fmt.Errorf("error getting certificate from the API server: request timed out - verify that Kubernetes certificate signer is setup, more at https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/#a-note-to-cluster-administrators")
}

func createSecret(certificate, key []byte) error {
	secretName := strings.Join([]string{prefix, "secret"}, "-")
	removeSecretIfExists(secretName)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: map[string][]byte{
			"cert.pem": certificate,
			"key.pem":  key,
		},
	}
	_, err := clientset.CoreV1().Secrets(namespace).Create(secret)
	return err
}

func createValidatingWebhookConfiguration(certificate []byte) error {
	configName := strings.Join([]string{prefix, "validating-config"}, "-")
	serviceName := strings.Join([]string{prefix, "service"}, "-")
	removeValidatingWebhookIfExists(configName)
	failurePolicy := arv1beta1.Ignore
	path := "/validate"
	configuration := &arv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
			Labels: map[string]string{
				"app": prefix,
			},
		},
		Webhooks: []arv1beta1.Webhook{
			arv1beta1.Webhook{
				Name: configName + ".k8s.cni.cncf.io",
				ClientConfig: arv1beta1.WebhookClientConfig{
					CABundle: certificate,
					Service: &arv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      &path,
					},
				},
				FailurePolicy: &failurePolicy,
				Rules: []arv1beta1.RuleWithOperations{
					arv1beta1.RuleWithOperations{
						Operations: []arv1beta1.OperationType{arv1beta1.Create},
						Rule: arv1beta1.Rule{
							APIGroups:   []string{"k8s.cni.cncf.io"},
							APIVersions: []string{"v1"},
							Resources:   []string{"network-attachment-definitions"},
						},
					},
				},
			},
		},
	}
	_, err := clientset.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(configuration)
	return err
}

func createService() error {
	serviceName := strings.Join([]string{prefix, "service"}, "-")
	removeServiceIfExists(serviceName)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Labels: map[string]string{
				"app": prefix,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Port:       443,
					TargetPort: intstr.FromInt(443),
				},
			},
			Selector: map[string]string{
				"app": prefix,
			},
		},
	}
	_, err := clientset.CoreV1().Services(namespace).Create(service)
	return err
}

func removeServiceIfExists(serviceName string) {
	service, err := clientset.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})
	if service != nil && err == nil {
		log.Printf("INFO: service %s already exists, removing it first", serviceName)
		err := clientset.CoreV1().Services(namespace).Delete(serviceName, &metav1.DeleteOptions{})
		if err != nil {
			fmt.Errorf("error trying to remove service: %s", err) // ?
		}
		log.Printf("INFO: service %s removed", serviceName)
	}
}

func removeValidatingWebhookIfExists(configName string) {
	validatingWebhok, err := clientset.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Get(configName, metav1.GetOptions{})
	if validatingWebhok != nil && err == nil {
		log.Printf("INFO: validating webhook %s already exists, removing it first", configName)
		err := clientset.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(configName, &metav1.DeleteOptions{})
		if err != nil {
			fmt.Errorf("error trying to remove validating webhook configuration: %s", err) // ?
		}
		log.Printf("INFO: validating webhook configuration %s removed", configName)
	}
}

func removeMutatingWebhookIfExists(configName string) {
	config, err := clientset.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(configName, metav1.GetOptions{})
	if config != nil && err == nil {
		log.Printf("INFO: mutating webhook %s already exists, removing it first", configName)
		err := clientset.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Delete(configName, &metav1.DeleteOptions{})
		if err != nil {
			log.Printf("ERROR: error trying to remove mutating webhook configuration: %s", err) // ?
		}
		log.Printf("INFO: mutating webhook configuration %s removed", configName)
	}
}

func removeSecretIfExists(secretName string) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if secret != nil && err == nil {
		log.Printf("secret %s already exists, removing it first", secretName)
		err := clientset.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
		if err != nil {
			log.Printf("ERROR: error trying to remove secret: %s", err) // ?
		}
		log.Printf("INFO: secret %s removed", secretName)
	}
}

func Install(k8sNamespace, namePrefix string) {
	/* setup Kubernetes API client */
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("FATAL: error loading Kubernetes in-cluster configuration: %s", err)
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("FATAL: error setting up Kubernetes client: %s", err)
	}

	namespace = k8sNamespace
	prefix = namePrefix

	/* generate CSR and private key */
	csr, key, err := generateCSR()
	if err != nil {
		log.Fatalf("FATAL: error generating CSR and private key: %s", err)
	}
	log.Printf("INFO: raw CSR and private key successfully created")

	/* obtain signed certificate */
	certificate, err := getSignedCertificate(csr)
	if err != nil {
		log.Fatalf("FATAL: error getting signed certificate: %s", err)
	}
	log.Printf("INFO: signed certificate successfully obtained")

	/* create secret and push it to the API */
	err = createSecret(certificate, key)
	if err != nil {
		log.Fatalf("FATAL: error creating secret: %s", err)
	}
	log.Printf("INFO: secret successfully created")

	/* create webhook configurations */
	err = createValidatingWebhookConfiguration(certificate)
	if err != nil {
		log.Fatalf("FATAL: error creating validating webhook configuration: %s", err)
	}
	log.Printf("INFO: validating webhook configuration successfully created")

	/* create service */
	err = createService()
	if err != nil {
		log.Fatalf("FATAL: error creating service: %s", err)
	}
	log.Printf("INFO: service successfully created")

	log.Printf("INFO: all resources created successfully")
}
