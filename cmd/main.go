package main

import (
	"fmt"

	"github.com/appscode/go/crypto/rand"
	framework "github.com/kubemove/kubemove/pkg/plugin/ddm/plugin"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"github.com/kubemove/elasticsearch-plugin/pkg/plugin"
)

const (
	PluginVersion = "v7.x"
)

func main() {
	var log = logf.Log.WithName(fmt.Sprintf("ECK Plugin %s", PluginVersion))

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "failed to build kubernetes client")
		utilruntime.Must(err)
	}

	err = framework.Register(rand.WithUniqSuffix("eck-plugin"), //TODO: Use stable name. Need fix in the server side.
		&plugin.ElasticsearchDDM{
			Log:       log,
			K8sClient: kubernetes.NewForConfigOrDie(config),
			DmClient:  dynamic.NewForConfigOrDie(config),
		})
	if err != nil {
		panic(err)
	}
}
