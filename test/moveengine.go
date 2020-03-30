package test

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	eck "github.com/elastic/cloud-on-k8s/pkg/apis/elasticsearch/v1"
	"github.com/kubemove/elasticsearch-plugin/pkg/plugin"
	"github.com/kubemove/kubemove/pkg/apis/kubemove/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var engineGVR = schema.GroupVersionResource{
	Group:    "kubemove.io",
	Version:  "v1alpha1",
	Resource: "moveengines",
}

const (
	KubemoveNamespace = "kubemove"
)

func (i *Invocation) newSampleMoveEngine(es *eck.Elasticsearch) (*v1alpha1.MoveEngine, error) {
	minioURL, err := i.getMinioServerAddress()
	if err != nil {
		return nil, err
	}
	pluginParameters := plugin.PluginParameters{
		Repository: plugin.RepositoryOptions{
			Name:        "minio_repo",
			Type:        "s3",
			Bucket:      i.testID,
			Prefix:      "es/backup",
			Endpoint:    minioURL,
			Scheme:      "http",
			Credentials: "minio-credentials",
		},
		Elasticsearch: plugin.ElasticsearchOptions{
			Name:        es.Name,
			Namespace:   es.Namespace,
			ServiceName: fmt.Sprintf("%s-es-http", es.Name),
			Scheme:      "https",
			Port:        9200,
			AuthSecret:  fmt.Sprintf("%s-es-elastic-user", es.Name),
			TLSSecret:   fmt.Sprintf("%s-es-http-ca-internal", es.Name),
		},
	}
	rawParameters, err := json.Marshal(&pluginParameters)
	if err != nil {
		return nil, err
	}

	return &v1alpha1.MoveEngine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MoveEngine",
			APIVersion: "kubemove.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.testID,
			Namespace: KubemoveNamespace,
		},
		Spec: v1alpha1.MoveEngineSpec{
			MovePair:         "local",
			Namespace:        KubemoveNamespace,
			RemoteNamespace:  KubemoveNamespace,
			SyncPeriod:       "*/3 * * * *",
			Mode:             "active",
			PluginProvider:   "elasticsearch-plugin",
			IncludeResources: false,
			PluginParameters: &runtime.RawExtension{
				Raw: rawParameters,
			},
		},
	}, nil
}

func (i *Invocation) createMoveEngine(engine *v1alpha1.MoveEngine) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(engine)
	if err != nil {
		return err
	}

	// Create MoveEngine in the source cluster
	_, err = i.SrcDmClient.Resource(engineGVR).Create(&unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	return err
}

func (i *Invocation) EventuallyStandbyMoveEngineCreated(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() bool {
		_, err := i.getMoveEngine(meta)
		if err != nil {
			return false
		}
		return true
	},
		DefaultTimeout,
		DefaultRetryInterval,
	)
}

func (i *Invocation) EventuallyMoveEngineReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() bool {
		engine, err := i.getMoveEngine(meta)
		if err != nil {
			return false
		}
		return engine.Status.Status == v1alpha1.MoveEngineReady
	},
		DefaultTimeout,
		DefaultRetryInterval,
	)
}

func (i *Invocation) EventuallySyncSucceeded(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() bool {
		engine, err := i.getMoveEngine(meta)
		if err != nil {
			return false
		}
		return engine.Status.DataSyncStatus == v1alpha1.SyncPhaseSynced
	},
		DefaultTimeout,
		DefaultRetryInterval,
	)
}

func (i *Invocation) getMoveEngine(meta metav1.ObjectMeta) (*v1alpha1.MoveEngine, error) {
	engine := &v1alpha1.MoveEngine{}
	obj, err := i.DstDmClient.Resource(engineGVR).Namespace(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, engine)
	if err != nil {
		return nil, err
	}
	return engine, nil
}

func (i *Invocation) deleteMoveEngine() error {
	err := i.SrcDmClient.Resource(engineGVR).Namespace(KubemoveNamespace).Delete(i.testID, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return i.DstDmClient.Resource(engineGVR).Namespace(KubemoveNamespace).Delete(i.testID, &metav1.DeleteOptions{})
}
