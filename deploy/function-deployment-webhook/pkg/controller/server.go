package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

type Config struct {
	schedulerName string
	certPath      string
	keyPath       string
	namespaces    []string
}

type Server struct {
	server *http.Server
	config *Config
}

func NewConfig(schedulerName, certPath, keyPath string, namespaces []string) *Config {
	return &Config{
		schedulerName: schedulerName,
		namespaces:    namespaces,
		certPath:      certPath,
		keyPath:       keyPath,
	}
}

func NewServer(c *Config) *Server {
	return &Server{
		server: &http.Server{
			Addr:           ":8443",
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1048576
		},
		config: c,
	}
}

func (s *Server) Start() {
	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)
	s.server.Handler = mux

	// start webhook server in new routine
	go func() {
		if err := s.server.ListenAndServeTLS(s.config.certPath, s.config.keyPath); err != nil {
			klog.Errorf("Failed to listen and serve webhook server: %v", err)
		}
	}()

}

func (s *Server) Shutdown() {
	s.server.Shutdown(context.Background())
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {

	klog.Info("Handling new request")

	// Read body contents
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		klog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// Verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		klog.Errorf("Can't decode body: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = s.admit(ar.Request)
	}

	admissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	klog.Infof("Ready to write reponse ...")
	klog.Info("Admission response: \n", string(resp))
	if _, err := w.Write(resp); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) admit(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {

	var deployment appsv1.Deployment
	if err := json.Unmarshal(request.Object.Raw, &deployment); err != nil {
		klog.Errorf("Could not unmarshal raw object: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Infof("AdmissionReview for Deployment %v", deployment)

	klog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		request.Kind, request.Namespace, request.Name, deployment.Name, request.UID, request.Operation, request.UserInfo)

	// Check if the deployment should be handled by the webhook
	for _, val := range s.config.namespaces {
		schedulerName, ok := deployment.Spec.Template.Labels["edgeautoscaler.polimi.it/scheduler"]
		if val == deployment.Namespace && ok {

			// Generate patch
			var patch []patchOperation
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  "/spec/template/spec/schedulerName",
				Value: schedulerName,
			})
			patchBytes, err := json.Marshal(patch)
			if err != nil {
				klog.Errorf("Failed to create patch with err: %s", err)
			}
			// Return the admission response with patch
			return &admissionv1.AdmissionResponse{
				Allowed: true,
				Result:  &metav1.Status{Status: "Success", Message: ""},
				Patch:   patchBytes,
				PatchType: func() *admissionv1.PatchType {
					pt := admissionv1.PatchTypeJSONPatch
					return &pt
				}(),
			}
		}
	}

	// Return the admission response without patch
	klog.Infof("Namespace %v not in the set of namespaces handled by the webhook", deployment.Namespace)
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{Status: "Success", Message: ""},
	}
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}
