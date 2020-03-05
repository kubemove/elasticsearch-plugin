package util

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/util/homedir"

	"github.com/elastic/go-elasticsearch/v7"

	"k8s.io/apimachinery/pkg/util/wait"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	common "github.com/elastic/cloud-on-k8s/pkg/apis/common/v1"
	eck "github.com/elastic/cloud-on-k8s/pkg/apis/elasticsearch/v1"
	esPlugin "github.com/kubemove/elasticsearch-plugin/pkg/plugin"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type PluginOptions struct {
	KubeConfigPath   string
	SrcContext       string
	DstContext       string
	SrcPluginAddress string
	DstPluginAddress string
	IndexName        string
	IndexFrom        string
	SrcClusterIp     string
	DstClusterIp     string
	SrcESNodePort    int64
	DstESNodePort    int64
	Debug            bool

	SrcKubeClient kubernetes.Interface
	DstKubeClient kubernetes.Interface
	SrcDmClient   dynamic.Interface
	DstDmClient   dynamic.Interface
}

// insertMinioRepository patches both source and destination Elasticsearches and inject s3-repository plugin installer init-container
func insertMinioRepository(dmClient dynamic.Interface) error {
	gvr := schema.GroupVersionResource{
		Group:    "elasticsearch.k8s.elastic.co",
		Version:  "v1",
		Resource: "elasticsearches",
	}

	es, err := getElasticsearch(dmClient, gvr)
	if err != nil {
		return err
	}

	// insert plugin installer init-container
	es.Spec.NodeSets[0].PodTemplate = corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: "install-plugins",
					Command: []string{
						"sh",
						"-c",
						"bin/elasticsearch-plugin install --batch repository-s3",
					},
				},
			},
		},
	}

	// insert minio credentiaals
	es.Spec.SecureSettings = []common.SecretSource{
		{
			SecretName: "minio-credentials",
		},
	}

	// convert updated Elasticsearch back to unstructured object
	updatedES, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&es)
	if err != nil {
		fmt.Println("Error during converting into unstructured object")
		return err
	}

	// update Elasticsearch
	_, err = dmClient.Resource(gvr).Namespace("default").Update(&unstructured.Unstructured{Object: updatedES}, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// give a delay for the ES phase to be updated
	time.Sleep(10 * time.Second)

	// wait for ES to be ready with the plugin installer
	err = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		es, err := getElasticsearch(dmClient, gvr)
		if err == nil {
			return es.Status.Phase == eck.ElasticsearchReadyPhase, nil
		}
		if !kerr.IsNotFound(err) {
			return true, err
		}
		return false, nil
	})
	return err
}

func getElasticsearch(dmClient dynamic.Interface, gvr schema.GroupVersionResource) (*eck.Elasticsearch, error) {
	// read Elasticsearch object
	resp, err := dmClient.Resource(gvr).Namespace("default").Get("sample-es", metav1.GetOptions{})
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

func (opt *PluginOptions) Setup() error {
	if opt.KubeConfigPath == "" {
		kubecfg := os.Getenv("KUBECONFIG")
		if kubecfg != "" {
			opt.KubeConfigPath = kubecfg
		} else {
			opt.KubeConfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
		}
	}

	loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: opt.KubeConfigPath}

	srcConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{CurrentContext: opt.SrcContext}).ClientConfig()
	if err != nil {
		return err
	}
	dstConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{CurrentContext: opt.DstContext}).ClientConfig()
	if err != nil {
		return err
	}

	opt.SrcKubeClient, err = kubernetes.NewForConfig(srcConfig)
	if err != nil {
		return err
	}
	opt.SrcDmClient, err = dynamic.NewForConfig(srcConfig)
	if err != nil {
		return err
	}

	opt.DstKubeClient, err = kubernetes.NewForConfig(dstConfig)
	if err != nil {
		return err
	}
	opt.DstDmClient, err = dynamic.NewForConfig(dstConfig)
	if err != nil {
		return err
	}
	return nil
}

func getESClient(kubeClient kubernetes.Interface, address string, port int32) (*elasticsearch.Client, error) {
	return esPlugin.NewElasticsearchClient(kubeClient, esPlugin.ElasticsearchOptions{
		ServiceName:        address,
		Namespace:          "default",
		Scheme:             "https",
		Port:               port,
		AuthSecret:         "sample-es-es-elastic-user",
		TLSSecret:          "sample-es-es-http-ca-internal",
		InsecureSkipVerify: true,
	})
}
