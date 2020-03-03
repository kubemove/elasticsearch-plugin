package plugin

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/go-logr/logr"
	framework "github.com/kubemove/kubemove/pkg/plugin/ddm/plugin"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/kubemove/kubemove/pkg/apis/kubemove/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"

	es "github.com/elastic/go-elasticsearch/v7"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
)

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
	ServiceName        string `json:"serviceName"`
	Namespace          string `json:"namespace"`
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

func NewElasticsearchClient(k8sClient kubernetes.Interface, opt ElasticsearchOptions) (*es.Client, error) {
	// configure client
	cfg := es.Config{
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

	return es.NewClient(cfg)
}

func configureBasicAuth(k8sClient kubernetes.Interface, cfg *es.Config, opt ElasticsearchOptions) error {
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

func configureTLS(k8sClient kubernetes.Interface, cfg *es.Config, opt ElasticsearchOptions) error {
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
