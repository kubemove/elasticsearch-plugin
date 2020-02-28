package plugin

import (
	"context"
	"fmt"
	"net/http"

	framework "github.com/kubemove/kubemove/pkg/plugin/ddm/plugin"

	"k8s.io/client-go/kubernetes"

	"github.com/elastic/go-elasticsearch/v7/esapi"
)

func (d *ElasticsearchDDM) Sync(params map[string]string, vol []*framework.Volume) (string, error) {
	fmt.Println("\nHandling SYNC request..................")
	// extract the plugin parameters from the respective MoveEngine
	pluginParameters, mode, err := extractPluginParameters(d.DmClient, params)
	if err != nil {
		return "", err
	}

	snapshotName, found := params[KeySnapshotName]
	if !found {
		return "", fmt.Errorf("snapshot name not found in the parameters")
	}

	// if it is source cluster, then trigger a snapshot
	if mode == EngineModeActive {
		fmt.Println("Triggering Snapshot: ", snapshotName)
		return triggerSnapshot(d.K8sClient, pluginParameters, snapshotName)
	} else {
		fmt.Println("Triggering Restore from Snapshot: ", snapshotName)
		// if it is destination cluster, then trigger a restore
		return triggerRestore(d.K8sClient, pluginParameters, snapshotName)
	}
}

// triggerSnapshot hits the Snapshot API of Elasticsearch to trigger a snapshot
func triggerSnapshot(k8sClient kubernetes.Interface, params PluginParameters, snapshotName string) (string, error) {
	// crate an Elasticsearch client
	esClient, err := NewElasticsearchClient(k8sClient, params.Elasticsearch)
	if err != nil {
		return "", err
	}

	// configure snapshot create request
	snapshotRequest := esapi.SnapshotCreateRequest{
		Repository: params.Repository.Name,
		Snapshot:   snapshotName,
		Pretty:     true,
	}

	// create snapshot
	resp, err := snapshotRequest.Do(context.Background(), esClient)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("Response: ", resp.String())

	if resp.StatusCode != http.StatusOK {
		// TODO: parse response and return failure case
		return "", fmt.Errorf("failed to create snapshot. Reason: <TO DO>")
	}

	return "", nil
}

// triggerRestore hits the Recovery API of Elasticsearch to trigger a restore process
func triggerRestore(k8sClient kubernetes.Interface, params PluginParameters, snapshotName string) (string, error) {
	// crate an Elasticsearch client
	esClient, err := NewElasticsearchClient(k8sClient, params.Elasticsearch)
	if err != nil {
		return "", err
	}

	// close all indexes
	fmt.Println("Requesting to close all indexes........")
	closeRequest := esapi.IndicesCloseRequest{
		Index:  []string{"_all"},
		Pretty: true,
	}
	closeResp, err := closeRequest.Do(context.Background(), esClient)
	if err != nil {
		return "", err
	}
	defer closeResp.Body.Close()

	fmt.Println("Response: ", closeResp.String())

	// trigger restore
	fmt.Println("Requesting to restore........")
	restoreRequest := esapi.SnapshotRestoreRequest{
		Repository: params.Repository.Name,
		Snapshot:   snapshotName,
		Pretty:     true,
	}
	resp, err := restoreRequest.Do(context.Background(), esClient)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("Response: ", resp.String())

	if resp.StatusCode != http.StatusOK {
		// TODO: parse response and return failure case
		return "", fmt.Errorf("failed to restore snapshot. Reason: <TO DO>")
	}

	return "", nil
}
