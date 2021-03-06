package util

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/util/homedir"

	"github.com/elastic/go-elasticsearch/v7"

	esPlugin "github.com/kubemove/elasticsearch-plugin/pkg/plugin"
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
	SrcClusterIp     string
	DstClusterIp     string
	SrcESNodePort    int64
	DstESNodePort    int64

	IndexName   string
	IndexFrom   string
	EsName      string
	EsNamespace string

	SrcKubeClient kubernetes.Interface
	DstKubeClient kubernetes.Interface
	SrcDmClient   dynamic.Interface
	DstDmClient   dynamic.Interface

	Debug bool
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

func getESClient(kubeClient kubernetes.Interface, address string, port int32, esName, esNamespace string) (*elasticsearch.Client, error) {
	return esPlugin.NewElasticsearchClient(kubeClient, esPlugin.ElasticsearchOptions{
		ServiceName:        address,
		Namespace:          esNamespace,
		Scheme:             "https",
		Port:               port,
		AuthSecret:         fmt.Sprintf("%s-es-elastic-user", esName),
		TLSSecret:          fmt.Sprintf("%s-es-http-ca-internal", esName),
		InsecureSkipVerify: true,
	})
}
