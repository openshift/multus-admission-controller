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
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/containernetworking/cni/libcni"
	"github.com/golang/glog"
	"github.com/intel/multus-cni/types"
	"github.com/pkg/errors"

	"k8s.io/api/admission/v1beta1"
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
)

var (
	clientset kubernetes.Interface
)

func validateNetworkAttachmentDefinition(netAttachDef types.NetworkAttachmentDefinition) (bool, error) {
	nameRegex := `^[a-z-1-9]([-a-z0-9]*[a-z0-9])?$`
	isNameCorrect, err := regexp.MatchString(nameRegex, netAttachDef.Metadata.Name)
	if !isNameCorrect {
		err := errors.New("net-attach-def name is invalid")
		glog.Info(err)
		return false, err
	}
	if err != nil {
		err := errors.Wrap(err, "error validating name")
		glog.Error(err)
		return false, err
	}

	if netAttachDef.Spec.Config == "" {
		err := errors.New("network config is empty")
		glog.Info(err)
		return false, err
	}

	glog.Infof("validating network config spec: %s", netAttachDef.Spec.Config)

	/* try to unmarshal config into NetworkConfig or NetworkConfigList
	   using actual code from libcni - if succesful, it means that the config
	   will be accepted by CNI itself as well */
	confBytes := []byte(netAttachDef.Spec.Config)
	_, err = libcni.ConfListFromBytes(confBytes)
	if err != nil {
		glog.Infof("spec is not a valid network config list: %s - trying to parse into standalone config", err)
		_, err = libcni.ConfFromBytes(confBytes)
		if err != nil {
			glog.Infof("spec is not a valid network config: %s", confBytes)
			err := errors.Wrap(err, "invalid config")
			return false, err
		}
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

func deserializeNetworkAttachmentDefinition(ar *v1beta1.AdmissionReview) (types.NetworkAttachmentDefinition, error) {
	/* unmarshal NetworkAttachmentDefinition from AdmissionReview request */
	netAttachDef := types.NetworkAttachmentDefinition{}
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
	glog.Infof("sending response to the Kubernetes API server")
	resp, _ := json.Marshal(ar)
	w.Write(resp)
}

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
