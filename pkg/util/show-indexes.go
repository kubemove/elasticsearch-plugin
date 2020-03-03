package util

import (
	"context"

	"github.com/elastic/go-elasticsearch/v7/esapi"

	esPlugin "github.com/kubemove/elasticsearch-plugin/pkg/plugin"
	"k8s.io/client-go/kubernetes"
)

// ShowIndexes shows all the indexes from the source/destination Elasticsearch based on "index-from" flag
// If the value of "index-from" flag is "active" then it shows indexes of the source ES.
// Otherwise, it shows indexes from the destination ES.
func (opt *PluginOptions) ShowIndexes() (string, error) {
	var address string
	var port int64
	var kubeClient kubernetes.Interface
	var err error

	if opt.IndexFrom == esPlugin.EngineModeActive {
		port = opt.SrcESNodePort
		address = opt.SrcClusterIp
		kubeClient = opt.SrcKubeClient
	} else {
		port = opt.DstESNodePort
		address = opt.DstClusterIp
		kubeClient = opt.DstKubeClient
	}

	client, err := getESClient(kubeClient, address, int32(port))
	if err != nil {
		return "", err
	}

	req := esapi.IndicesGetRequest{
		Index:  []string{"_all"},
		Pretty: true,
	}

	resp, err := req.Do(context.Background(), client)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return resp.String(), nil
}
