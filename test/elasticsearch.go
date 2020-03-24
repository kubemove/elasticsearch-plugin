package test

import (
	"github.com/appscode/go/crypto/rand"
	eckCommon "github.com/elastic/cloud-on-k8s/pkg/apis/common/v1"
	eck "github.com/elastic/cloud-on-k8s/pkg/apis/elasticsearch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func createElasticsearch(es *eck.Elasticsearch) error {
	// TODO: complete this function
	return nil
}