package plugin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"

	"github.com/elastic/go-elasticsearch/v7/esapi"
)

func (d *ElasticsearchDDM) Sync(params map[string]string) (string, error) {
	fmt.Println("\nHandling SYNC request..................")
	// extract the plugin parameters from the respective MoveEngine
	pluginParameters, mode, err := extractPluginParameters(d.DmClient, params)
	if err != nil {
		d.Log.Error(err, "failed to extract plugin parameters")
		return "", err
	}

	snapshotName, found := params[KeySnapshotName]
	if !found {
		return "", fmt.Errorf("snapshot name not found in the parameters")
	}

	// if it is source cluster, then trigger a snapshot
	if mode == EngineModeActive {
		fmt.Println("Triggering Snapshot: ", snapshotName)
		res, err := triggerSnapshot(d.K8sClient, pluginParameters, snapshotName)
		if err != nil {
			d.Log.Error(err, "failed to trigger snapshot process")
		}
		return res, err
	} else {
		fmt.Println("Triggering Restore from Snapshot: ", snapshotName)
		// if it is destination cluster, then trigger a restore
		res, err := triggerRestore(d.K8sClient, pluginParameters, snapshotName)
		if err != nil {
			d.Log.Error(err, "failed to trigger restore process")
		}
		return res, err
	}
}

// triggerSnapshot hits the Snapshot API of Elasticsearch to trigger a snapshot
// nolint: unparam
func triggerSnapshot(k8sClient kubernetes.Interface, params PluginParameters, snapshotName string) (string, error) {
	// crate an Elasticsearch client
	esClient, err := NewElasticsearchClient(k8sClient, params.Elasticsearch)
	if err != nil {
		return "", errors.Wrap(err, "failed to crate Elasticsearch client")
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
		return "", errors.Wrap(err, "failed to send SnapshotCreateRequest")
	}
	defer resp.Body.Close()

	fmt.Println("Response: ", resp.String())

	if resp.StatusCode != http.StatusOK {
		rootCause, err := parseErrorCause(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to trigger snapshot.\n"+
				"Also, failed to parse the error info. Reason: %s", err.Error())
		}
		return "", fmt.Errorf("failed to trigger snapshot.\n"+
			"Error Type: %s, Reason: %s", rootCause.Type, rootCause.Reason)
	}

	return "", nil
}

// triggerRestore hits the Recovery API of Elasticsearch to trigger a restore process
// nolint: unparam
func triggerRestore(k8sClient kubernetes.Interface, params PluginParameters, snapshotName string) (string, error) {
	// crate an Elasticsearch client
	esClient, err := NewElasticsearchClient(k8sClient, params.Elasticsearch)
	if err != nil {
		return "", errors.Wrap(err, "failed to create Elasticsearch client")
	}

	// close all indexes
	fmt.Println("Requesting to close all indexes........")
	closeRequest := esapi.IndicesCloseRequest{
		Index:  []string{"_all"},
		Pretty: true,
	}
	closeResp, err := closeRequest.Do(context.Background(), esClient)
	if err != nil {
		return "", errors.Wrap(err, "failed to send IndiciesCloseRequest")
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
		return "", errors.Wrap(err, "failed to send SnapshotRestoreRequest")
	}
	defer resp.Body.Close()

	fmt.Println("Response: ", resp.String())

	if resp.StatusCode != http.StatusOK {
		rootCause, err := parseErrorCause(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to trigger restore.\n"+
				"Also, failed to parse the error info. Reason: %s", err.Error())
		}
		return "", fmt.Errorf("failed to trigger restore.\n"+
			"Error Type: %s, Reason: %s", rootCause.Type, rootCause.Reason)
	}

	return "", nil
}
