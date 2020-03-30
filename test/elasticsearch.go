package test

import (
	"github.com/appscode/go/crypto/rand"
	eckCommon "github.com/elastic/cloud-on-k8s/pkg/apis/common/v1"
	eck "github.com/elastic/cloud-on-k8s/pkg/apis/elasticsearch/v1"
	"github.com/kubemove/elasticsearch-plugin/pkg/plugin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func newDefaultElasticsearch() *eck.Elasticsearch {
	return &eck.Elasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Name: rand.WithUniqSuffix("sample-es"),
		},
		Spec: eck.ElasticsearchSpec{
			Version: "7.5.2",
			HTTP: eckCommon.HTTPConfig{
				Service: eckCommon.ServiceTemplate{
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
					},
				},
			},
			NodeSets: []eck.NodeSet{
				{
					Name:  "default",
					Count: 2,
				},
			},
		},
	}
}

func (i *Invocation) createElasticsearch(es *eck.Elasticsearch) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(es)
	if err != nil {
		return err
	}

	// Create Elasticsearch in the source cluster
	_, err = i.SrcDmClient.Resource(plugin.ESGVR).Create(&unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Create Elasticsearch in the destination cluster
	_, err = i.DstDmClient.Resource(plugin.ESGVR).Create(&unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	params := plugin.PluginParameters{
		Elasticsearch: plugin.ElasticsearchOptions{
			Name:      es.Name,
			Namespace: es.Namespace,
		},
	}
	// Wait for source Elasticsearch to be ready
	err = plugin.WaitUntilElasticsearchReady(i.SrcKubeClient, i.SrcDmClient, params)
	if err != nil {
		return err
	}
	// Wait for destination Elasticsearch to be ready
	return plugin.WaitUntilElasticsearchReady(i.DstKubeClient, i.DstDmClient, params)
}
