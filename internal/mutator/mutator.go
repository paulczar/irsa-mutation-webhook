/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package mutator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	"kubevirt.io/irsa-mutation-webhook/pkg/config"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

const (
	irsaAnnotation = "eks.amazonaws.com/role-arn"
)

type Mutator struct {
	config    *config.Config
	k8sClient *kubernetes.Clientset
}

func NewMutator(cfg *config.Config) (*Mutator, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
	}

	k8sClient, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	return &Mutator{
		config:    cfg,
		k8sClient: k8sClient,
	}, nil
}

func (m *Mutator) HandleMutate(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "invalid Content-Type, want `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	admissionReview := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &admissionReview); err != nil {
		http.Error(w, fmt.Sprintf("could not decode body: %v", err), http.StatusBadRequest)
		return
	}

	if admissionReview.Request == nil || admissionReview.Request.UID == "" {
		http.Error(w, "AdmissionReview request UID is required", http.StatusBadRequest)
		return
	}

	admissionResponse := m.mutate(admissionReview.Request)

	admissionReview.Response = admissionResponse
	admissionReview.APIVersion = "admission.k8s.io/v1"
	admissionReview.Kind = "AdmissionReview"
	admissionReview.Response.UID = admissionReview.Request.UID

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

func (m *Mutator) mutate(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if request.Kind.Kind != "Pod" {
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	var pod corev1.Pod
	if err := json.Unmarshal(request.Object.Raw, &pod); err != nil {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("could not unmarshal pod object: %v", err),
			},
		}
	}

	if !isKubeVirtPod(&pod) {
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	sa, err := m.getServiceAccount(pod.Namespace, pod.Spec.ServiceAccountName)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to get service account: %v", err),
			},
		}
	}

	roleARN, hasIRSA := sa.Annotations[irsaAnnotation]
	if !hasIRSA {
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	patch, err := m.createVirtioFSPatch(&pod, roleARN)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to create patch: %v", err),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patch,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (m *Mutator) createVirtioFSPatch(pod *corev1.Pod, roleARN string) ([]byte, error) {
	virtiofsContainer := corev1.Container{
		Name:            "virtiofs-aws-iam-token",
		Image:           m.config.VirtioFSImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/usr/libexec/virtiofsd"},
		Args: []string{
			"--socket-path=/var/run/kubevirt/virtiofs-containers/aws-iam-token.sock",
			"--shared-dir=/var/run/secrets/eks.amazonaws.com/serviceaccount",
			"--sandbox=none",
			"--cache=auto",
			"--migration-on-error=guest-error",
			"--migration-mode=find-paths",
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("1M"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "virtiofs-containers",
				MountPath: "/var/run/kubevirt/virtiofs-containers",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                ptr.To[int64](107),
			RunAsGroup:               ptr.To[int64](107),
			RunAsNonRoot:             ptr.To(true),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
	}

	patch := []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/spec/containers/-",
			"value": virtiofsContainer,
		},
	}

	return json.Marshal(patch)
}

func (m *Mutator) getServiceAccount(namespace, name string) (*corev1.ServiceAccount, error) {
	if name == "" {
		name = "default"
	}
	return m.k8sClient.CoreV1().ServiceAccounts(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func isKubeVirtPod(pod *corev1.Pod) bool {
	_, hasLabel := pod.Labels["kubevirt.io"]
	if hasLabel {
		return true
	}

	for k := range pod.Labels {
		if strings.HasPrefix(k, "kubevirt.io") || strings.HasPrefix(k, "vm.kubevirt.io") {
			return true
		}
	}

	return false
}
