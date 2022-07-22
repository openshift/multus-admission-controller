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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/golang/glog"
	"gopkg.in/k8snetworkplumbingwg/multus-cni.v3/pkg/types"
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/pkg/errors"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type jsonPatchOperation struct {
	Operation string      `json:"op"`
	Path      string      `json:"path"`
	Value     interface{} `json:"value,omitempty"`
}

const (
	networksAnnotationKey  = "k8s.v1.cni.cncf.io/networks"
	networkResourceNameKey = "k8s.v1.cni.cncf.io/resourceName"
	namespaceConstraint    = "_local"
)

var (
	clientset kubernetes.Interface
)

// validateCNIConfig verifies following fields
// conf: 'type'
// conflist: 'plugins' and 'type'
func validateCNIConfig(config []byte) error {
	var c map[string]interface{}
	if err := json.Unmarshal(config, &c); err != nil {
		return err
	}

	// Identify target is single CNI config or plugins
	if p, ok := c["plugins"]; ok {
		// CNI conflist
		// check 'type' field for each plugin in 'plugins'
		plugins := p.([]interface{})
		for _, v := range plugins {
			plugin := v.(map[string]interface{})
			if _, ok := plugin["type"]; !ok {
				return fmt.Errorf("missing 'type' in plugins")
			}
		}
	} else {
		// single CNI config
		if _, ok := c["type"]; !ok {
			return fmt.Errorf("missing 'type' in cni config")
		}
	}
	return nil
}

// preprocessCNIConfig process CNI config bytes as following (that multus does too)
// - if 'name' is missing, 'name' is filled
func preprocessCNIConfig(name string, config []byte) ([]byte, error) {
	var c map[string]interface{}
	if err := json.Unmarshal(config, &c); err != nil {
		if n, ok := c["name"]; !ok || n == "" {
			c["name"] = name
		}
	}
	configBytes, err := json.Marshal(c)
	return configBytes, err
}

// isJSON detects if a string is in JSON format
func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func validateNetworkAttachmentDefinition(netAttachDef netv1.NetworkAttachmentDefinition) (bool, error) {
	nameRegex := `^[a-z-1-9]([-a-z0-9]*[a-z0-9])?$`
	isNameCorrect, err := regexp.MatchString(nameRegex, netAttachDef.GetName())
	if !isNameCorrect {
		err := errors.New("net-attach-def name is invalid")
		glog.Info(err)
		return false, err
	}
	if err != nil {
		err := errors.New("error validating name")
		glog.Error(err)
		return false, err
	}

	glog.Infof("validating network config spec: %s", netAttachDef.Spec.Config)

	var confBytes []byte
	if netAttachDef.Spec.Config != "" {
		// try to unmarshal config into NetworkConfig or NetworkConfigList
		//  using actual code from libcni - if succesful, it means that the config
		//  will be accepted by CNI itself as well
		if !isJSON(netAttachDef.Spec.Config) {
			err := errors.New("configuration string is not in JSON format")
			glog.Info(err)
			return false, err
		}

		confBytes, err = preprocessCNIConfig(netAttachDef.GetName(), []byte(netAttachDef.Spec.Config))
		if err != nil {
			err := errors.New("invalid json")
			return false, err
		}
		if err := validateCNIConfig(confBytes); err != nil {
			err := errors.New("invalid config")
			return false, err
		}
		_, err = libcni.ConfListFromBytes(confBytes)
		if err != nil {
			glog.Infof("spec is not a valid network config list: %s - trying to parse into standalone config", err)
			_, err = libcni.ConfFromBytes(confBytes)
			if err != nil {
				glog.Infof("spec is not a valid network config: %s", confBytes)
				err := errors.New("invalid config")
				return false, err
			}
		}

	} else {
		glog.Infof("Allowing empty spec.config")
	}

	glog.Infof("AdmissionReview request allowed: Network Attachment Definition '%s' is valid", confBytes)
	return true, nil
}

func prepareAdmissionReviewResponse(allowed bool, message string, ar *v1beta1.AdmissionReview) error {
	if ar.Request != nil {
		ar.Response = &v1beta1.AdmissionResponse{
			UID:     ar.Request.UID,
			Allowed: allowed,
		}
		if message != "" {
			ar.Response.Result = &metav1.Status{
				Message: message,
			}
		}
		return nil
	}
	return errors.New("received empty AdmissionReview request")
}

func readAdmissionReview(req *http.Request) (*v1beta1.AdmissionReview, int, error) {
	var body []byte

	if req.Body != nil {
		if data, err := ioutil.ReadAll(req.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		err := errors.New("Error reading HTTP request: empty body")
		glog.Error(err)
		return nil, http.StatusBadRequest, err
	}

	/* validate HTTP request headers */
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		err := errors.Errorf("Invalid Content-Type='%s', expected 'application/json'", contentType)
		glog.Error(err)
		return nil, http.StatusUnsupportedMediaType, err
	}

	/* read AdmissionReview from the request body */
	ar, err := deserializeAdmissionReview(body)
	if err != nil {
		err := errors.Wrap(err, "error deserializing AdmissionReview")
		glog.Error(err)
		return nil, http.StatusBadRequest, err
	}

	return ar, http.StatusOK, nil
}

func deserializeAdmissionReview(body []byte) (*v1beta1.AdmissionReview, error) {
	ar := &v1beta1.AdmissionReview{}
	runtimeScheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(runtimeScheme)
	deserializer := codecs.UniversalDeserializer()
	_, _, err := deserializer.Decode(body, nil, ar)

	/* Decode() won't return an error if the data wasn't actual AdmissionReview */
	if err == nil && ar.TypeMeta.Kind != "AdmissionReview" {
		err = errors.New("received object is not an AdmissionReview")
	}

	return ar, err
}

func analyzeIsolationAnnotation(ar *v1beta1.AdmissionReview) (bool, error) {

	var metadata *metav1.ObjectMeta
	var pod v1.Pod

	req := ar.Request

	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		return false, err
	}

	metadata = &pod.ObjectMeta
	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	if len(annotations[networksAnnotationKey]) > 0 {

		glog.Infof("Analyzing %s annotation: %s", networksAnnotationKey, annotations[networksAnnotationKey])

		networks, err := parsePodNetworkAnnotation(annotations[networksAnnotationKey], namespaceConstraint)
		if err != nil {
			glog.Errorf("Error during parsePodNetworkAnnotation: %v", err)
			return false, err
		}

		for _, item := range networks {
			fmt.Printf("name: %v", item.Namespace)
			if item.Namespace != namespaceConstraint {
				annotationerrorstring := fmt.Sprintf("%s annotations must not refer to namespaced values (must use local namespace, i.e. must not contain a /), rejected: %s (namespace: %s)", networksAnnotationKey, annotations[networksAnnotationKey], item.Namespace)
				annotationerror := errors.New(annotationerrorstring)
				return false, annotationerror
			}
		}

		glog.Infof("Allowed value: %s", annotations[networksAnnotationKey])

	}

	return true, nil

}

func parsePodNetworkAnnotation(podNetworks, defaultNamespace string) ([]*types.NetworkSelectionElement, error) {
	var networks []*types.NetworkSelectionElement

	// logging.Debugf("parsePodNetworkAnnotation: %s, %s", podNetworks, defaultNamespace)
	if podNetworks == "" {
		return nil, fmt.Errorf("parsePodNetworkAnnotation: pod annotation not having \"network\" as key, refer Multus README.md for the usage guide")
	}

	if strings.IndexAny(podNetworks, "[{\"") >= 0 {
		if err := json.Unmarshal([]byte(podNetworks), &networks); err != nil {
			return nil, fmt.Errorf("parsePodNetworkAnnotation: failed to parse pod Network Attachment Selection Annotation JSON format: %v", err)
		}
	} else {
		// Comma-delimited list of network attachment object names
		for _, item := range strings.Split(podNetworks, ",") {
			// Remove leading and trailing whitespace.
			item = strings.TrimSpace(item)

			// Parse network name (i.e. <namespace>/<network name>@<ifname>)
			netNsName, networkName, netIfName, err := parsePodNetworkObjectName(item)
			if err != nil {
				return nil, fmt.Errorf("parsePodNetworkAnnotation: %v", err)
			}

			networks = append(networks, &types.NetworkSelectionElement{
				Name:             networkName,
				Namespace:        netNsName,
				InterfaceRequest: netIfName,
			})
		}
	}

	for _, net := range networks {
		if net.Namespace == "" {
			net.Namespace = defaultNamespace
		}
	}

	return networks, nil
}

func parsePodNetworkObjectName(podnetwork string) (string, string, string, error) {
	var netNsName string
	var netIfName string
	var networkName string

	// logging.Debugf("parsePodNetworkObjectName: %s", podnetwork)
	slashItems := strings.Split(podnetwork, "/")
	if len(slashItems) == 2 {
		netNsName = strings.TrimSpace(slashItems[0])
		networkName = slashItems[1]
	} else if len(slashItems) == 1 {
		networkName = slashItems[0]
	} else {
		return "", "", "", fmt.Errorf("Invalid network object (failed at '/')")
	}

	atItems := strings.Split(networkName, "@")
	networkName = strings.TrimSpace(atItems[0])
	if len(atItems) == 2 {
		netIfName = strings.TrimSpace(atItems[1])
	} else if len(atItems) != 1 {
		return "", "", "", fmt.Errorf("Invalid network object (failed at '@')")
	}

	// Check and see if each item matches the specification for valid attachment name.
	// "Valid attachment names must be comprised of units of the DNS-1123 label format"
	// [a-z0-9]([-a-z0-9]*[a-z0-9])?
	// And we allow at (@), and forward slash (/) (units separated by commas)
	// It must start and end alphanumerically.
	allItems := []string{netNsName, networkName, netIfName}
	for i := range allItems {
		matched, _ := regexp.MatchString("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", allItems[i])
		if !matched && len([]rune(allItems[i])) > 0 {
			return "", "", "", fmt.Errorf(fmt.Sprintf("Failed to parse: one or more items did not match comma-delimited format (must consist of lower case alphanumeric characters). Must start and end with an alphanumeric character), mismatch @ '%v'", allItems[i]))
		}
	}

	// logging.Debugf("parsePodNetworkObjectName: parsed: %s, %s, %s", netNsName, networkName, netIfName)
	return netNsName, networkName, netIfName, nil
}

func deserializeNetworkAttachmentDefinition(ar *v1beta1.AdmissionReview) (netv1.NetworkAttachmentDefinition, error) {
	/* unmarshal NetworkAttachmentDefinition from AdmissionReview request */
	netAttachDef := netv1.NetworkAttachmentDefinition{}
	err := json.Unmarshal(ar.Request.Object.Raw, &netAttachDef)
	return netAttachDef, err
}

func handleValidationError(w http.ResponseWriter, ar *v1beta1.AdmissionReview, orgErr error) {
	err := prepareAdmissionReviewResponse(false, orgErr.Error(), ar)
	if err != nil {
		err := errors.Wrap(err, "error preparing AdmissionResponse")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeResponse(w, ar)
}

func writeResponse(w http.ResponseWriter, ar *v1beta1.AdmissionReview) {
	// glog.Infof("sending response to the Kubernetes API server")
	resp, _ := json.Marshal(ar)
	w.Write(resp)
}

// IsolateHandler Handles namespace isolation validation.
func IsolateHandler(w http.ResponseWriter, req *http.Request) {

	var allowed bool

	ar, httpStatus, err := readAdmissionReview(req)
	if err != nil {
		http.Error(w, err.Error(), httpStatus)
		return
	}

	allowed, err = analyzeIsolationAnnotation(ar)
	if err != nil {
		handleValidationError(w, ar, err)
		return
	}

	err = prepareAdmissionReviewResponse(allowed, "", ar)
	if err != nil {
		glog.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeResponse(w, ar)
}

// ValidateHandler handles net-attach-def validation requests
func ValidateHandler(w http.ResponseWriter, req *http.Request) {
	/* read AdmissionReview from the HTTP request */
	ar, httpStatus, err := readAdmissionReview(req)
	if err != nil {
		http.Error(w, err.Error(), httpStatus)
		return
	}

	netAttachDef, err := deserializeNetworkAttachmentDefinition(ar)
	if err != nil {
		handleValidationError(w, ar, err)
		return
	}

	/* perform actual object validation */
	allowed, err := validateNetworkAttachmentDefinition(netAttachDef)
	if err != nil {
		handleValidationError(w, ar, err)
		return
	}

	/* perpare response and send it back to the API server */
	err = prepareAdmissionReviewResponse(allowed, "", ar)
	if err != nil {
		glog.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeResponse(w, ar)
}

// SetupInClusterClient sets up api configuration
func SetupInClusterClient() {
	/* setup Kubernetes API client */
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}
}
