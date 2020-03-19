package plugin

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	framework "github.com/kubemove/kubemove/pkg/plugin/ddm/plugin"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/kubemove/kubemove/pkg/apis/kubemove/v1alpha1"

	esv7 "github.com/elastic/go-elasticsearch/v7"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/runtime/schema"

	common "github.com/elastic/cloud-on-k8s/pkg/apis/common/v1"
	eck "github.com/elastic/cloud-on-k8s/pkg/apis/elasticsearch/v1"
	appsv1 "k8s.io/api/apps/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

const (
	MoveEngineName      = "engineName"
	MoveEngineNamespace = "engineNamespace"

	RepositoryParameters   = "repository"
	ElasticsearchParameter = "elasticsearch"

	EngineModeActive = "active"
	ElasticUser      = "elastic"
	TLSCertKey       = "tls.crt"
	KeySnapshotName  = "snapshotName"

	ContainerPluginInstaller = "plugin-installer"
)

var esGVR = schema.GroupVersionResource{
	Group:    "elasticsearch.k8s.elastic.co",
	Version:  "v1",
	Resource: "elasticsearches",
}

type ElasticsearchDDM struct {
	Log       logr.Logger
	P         framework.Plugin
	K8sClient kubernetes.Interface
	DmClient  dynamic.Interface
}

type PluginParameters struct {
	Repository    RepositoryOptions    `json:"repository"`
	Elasticsearch ElasticsearchOptions `json:"elasticsearch"`
}

type RepositoryOptions struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Bucket      string `json:"bucket"`
	Prefix      string `json:"prefix,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Scheme      string `json:"scheme"`
	Credentials string `json:"credentials"`
}

type ElasticsearchOptions struct {
	Name               string `json:"name"`
	Namespace          string `json:"namespace"`
	ServiceName        string `json:"serviceName"`
	Scheme             string `json:"scheme"`
	Port               int32  `json:"port"`
	AuthSecret         string `json:"authSecret"`
	TLSSecret          string `json:"tlsSecret,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
}

type ErrorInfo struct {
	Error ErrorCause `json:"error,omitempty"`
}
type ErrorCause struct {
	RootCause []RootCause `json:"root_cause,omitempty"`
}
type RootCause struct {
	Type   string `json:"type,omitempty"`
	Reason string `json:"reason,omitempty"`
}

var _ framework.Plugin = (*ElasticsearchDDM)(nil)

func NewElasticsearchClient(k8sClient kubernetes.Interface, opt ElasticsearchOptions) (*esv7.Client, error) {
	// configure client
	cfg := esv7.Config{
		Addresses: []string{fmt.Sprintf("%s://%s:%d", opt.Scheme, opt.ServiceName, opt.Port)},
	}

	// configure authentication
	err := configureBasicAuth(k8sClient, &cfg, opt)
	if err != nil {
		return nil, err
	}

	if opt.Scheme == "https" {
		err = configureTLS(k8sClient, &cfg, opt)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return esv7.NewClient(cfg)
}

func configureBasicAuth(k8sClient kubernetes.Interface, cfg *esv7.Config, opt ElasticsearchOptions) error {
	// get the elastic user secret
	authSecret, err := k8sClient.CoreV1().Secrets(opt.Namespace).Get(opt.AuthSecret, metav1.GetOptions{})
	if err != nil {
		return err
	}

	password, ok := authSecret.Data[ElasticUser]
	if !ok {
		return fmt.Errorf("failed to set Authorization Headers. Reason: no passowrd found for %q user", ElasticUser)
	}

	cfg.Username = ElasticUser
	cfg.Password = string(password)

	return nil
}

func configureTLS(k8sClient kubernetes.Interface, cfg *esv7.Config, opt ElasticsearchOptions) error {
	// get the internal cert secret
	certSecret, err := k8sClient.CoreV1().Secrets(opt.Namespace).Get(opt.TLSSecret, metav1.GetOptions{})
	if err != nil {
		return err
	}

	caCert, found := certSecret.Data[TLSCertKey]
	if !found {
		return fmt.Errorf("failed to set TLS transport. Reason: tls.crt not found in secret %s/%s", opt.Namespace, opt.TLSSecret)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opt.InsecureSkipVerify,
			RootCAs:            caCertPool,
		},
	}
	cfg.Transport = tr

	return nil
}

func extractPluginParameters(dmClient dynamic.Interface, params map[string]string) (PluginParameters, string, error) {
	parameters := PluginParameters{}

	engineName, found := params[MoveEngineName]
	if !found {
		return parameters, "", fmt.Errorf("failed to extract plugin's parameters. Reason: MoveEngine name not found in the paramters")
	}

	engineNamespace, found := params[MoveEngineNamespace]
	if !found {
		return parameters, "", fmt.Errorf("failed to extract plugin's parameters. Reason: MoveEngine namespace not found in the paramters")
	}

	// read MoveEngine CR as unstructured object using dynamic client
	moveEngineGVR := v1alpha1.SchemeGroupVersion.WithResource(v1alpha1.ResourcePluralMoveEngine)
	resp, err := dmClient.Resource(moveEngineGVR).Namespace(engineNamespace).Get(engineName, metav1.GetOptions{})
	if err != nil {
		return parameters, "", err
	}

	// convert unstructured object into MoveEngine CR
	var moveEngine v1alpha1.MoveEngine
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(resp.UnstructuredContent(), &moveEngine)
	if err != nil {
		return parameters, "", err
	}

	if moveEngine.Spec.PluginParameters != nil {
		if err := json.Unmarshal(moveEngine.Spec.PluginParameters.Raw, &parameters); err != nil {
			return parameters, "", fmt.Errorf("failed to Unmarshal plugin parameters")
		}
	} else {
		return parameters, "", fmt.Errorf("plugin parameters not found in the MoveEngine")
	}

	return parameters, moveEngine.Spec.Mode, nil
}

func parseErrorCause(body io.ReadCloser) (*RootCause, error) {
	var errorInfo ErrorInfo
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &errorInfo)
	if err != nil {
		return nil, err
	}
	return &errorInfo.Error.RootCause[0], nil
}

// insertMinioRepository patches both source and destination Elasticsearches and inject s3-repository plugin installer init-container
func insertMinioRepository(k8sClient kubernetes.Interface, dmClient dynamic.Interface, params PluginParameters) error {
	es, err := getElasticsearch(dmClient, params)
	if err != nil {
		return err
	}

	// insert plugin installer init-container
	es.Spec.NodeSets[0].PodTemplate = corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: ContainerPluginInstaller,
					Command: []string{
						"sh",
						"-c",
						"bin/elasticsearch-plugin install --batch repository-s3",
					},
				},
			},
		},
	}

	// insert minio credentials
	es.Spec.SecureSettings = []common.SecretSource{
		{
			SecretName: params.Repository.Credentials,
		},
	}

	// convert updated Elasticsearch back to unstructured object
	updatedES, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&es)
	if err != nil {
		return errors.Wrap(err, "failed to convert updated Elasticsearch into Unstructured object")
	}

	// update Elasticsearch
	_, err = dmClient.Resource(esGVR).Namespace(params.Elasticsearch.Namespace).Update(&unstructured.Unstructured{Object: updatedES}, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// wait for ES to be ready with the plugin installer
	return WaitUntilElasticsearchReady(k8sClient, dmClient, params)
}

func WaitUntilElasticsearchReady(k8sClient kubernetes.Interface, dmClient dynamic.Interface, params PluginParameters) error {
	err := wait.PollImmediate(5*time.Second, 10*time.Minute, func() (done bool, err error) {
		es, err := getElasticsearch(dmClient, params)
		if err == nil && es.Status.Phase == eck.ElasticsearchReadyPhase {
			patchApplied, err2 := checkPatchState(k8sClient, es)
			if err2 != nil {
				return true, err
			}
			return patchApplied, nil
		}
		if !kerr.IsNotFound(err) {
			return true, err
		}
		return false, nil
	})

	return err
}

func getElasticsearch(dmClient dynamic.Interface, params PluginParameters) (*eck.Elasticsearch, error) {
	// read Elasticsearch object
	resp, err := dmClient.Resource(esGVR).Namespace(params.Elasticsearch.Namespace).Get(params.Elasticsearch.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// convert to unstructured object into Elasticsearch type
	var es eck.Elasticsearch
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(resp.UnstructuredContent(), &es)
	if err != nil {
		return nil, err
	}
	return &es, nil
}

func checkPatchState(k8sClient kubernetes.Interface, es *eck.Elasticsearch) (bool, error) {
	// Identify the respective StatefulSets
	sts, err := k8sClient.AppsV1().StatefulSets(es.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for i := range sts.Items {
		fmt.Println("StatefulSet: ", sts.Items[i].Name)
		if metav1.IsControlledBy(&sts.Items[i], es) && patchAppliedToPods(k8sClient, sts.Items[i]) {
			return true, nil
		}
	}
	return false, fmt.Errorf("no StatefulSet found for Elasticsearch %s/%s", es.Namespace, es.Name)
}

func patchAppliedToPods(k8sClient kubernetes.Interface, s appsv1.StatefulSet) bool {
	fmt.Printf("Checking patch status for StatefulSet: %s/%s", s.Namespace, s.Name)

	pods, err := k8sClient.CoreV1().Pods(s.Namespace).List(metav1.ListOptions{LabelSelector: s.Spec.Selector.String()})
	if err != nil {
		return false
	}
	if len(pods.Items) != int(*s.Spec.Replicas) {
		return false
	}

	allPodHasPluginInstaller := true
	for _, pod := range pods.Items {
		hasPluginInstaller := false
		for _, c := range pod.Spec.InitContainers {
			if c.Name == ContainerPluginInstaller {
				hasPluginInstaller = true
			}
		}
		if !hasPluginInstaller {
			allPodHasPluginInstaller = false
		}
	}
	return allPodHasPluginInstaller
}
