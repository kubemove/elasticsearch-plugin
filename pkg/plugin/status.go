package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"

	"github.com/elastic/go-elasticsearch/v7/esapi"
	framework "github.com/kubemove/kubemove/pkg/plugin/ddm/plugin"
)

const (
	SnapshotSucceeded  = "SUCCESS"
	SnapshotInProgress = "IN_PROGRESS"
	SnapshotFailed     = "FAILED"

	RecoveryTypeSnapshot = "SNAPSHOT"
	RecoveryStageDone    = "DONE"
)

// SnapshotGetResponse is used to unmarshal the response body of SnapshotGetRequest
type SnapshotGetResponse struct {
	Snapshots []SnapshotStatus `json:"snapshots"`
}
type SnapshotStatus struct {
	Snapshot string `json:"snapshot"`
	State    string `json:"state"`
}

// IndexesRecoveryStatus is used to unmarshal the indexes recovery status from IndicesRecoveryRequest response
type IndexesRecoveryStatus struct {
	Shards []ShardRecoveryStatus `json:"shards"`
}
type ShardRecoveryStatus struct {
	Type   string         `json:"type"`
	Stage  string         `json:"stage"`
	Source RecoverySource `json:"source"`
}
type RecoverySource struct {
	Snapshot string `json:"snapshot,omitempty"`
}

func (d *ElasticsearchDDM) Status(params map[string]string) (int32, error) {
	fmt.Println("\nHandling STATUS request..................")
	// extract the plugin parameters from the respective MoveEngine
	pluginParameters, mode, err := extractPluginParameters(d.DmClient, params)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed to extract plugin parameters")
	}

	snapshotName, found := params[KeySnapshotName]
	if !found {
		return framework.Errored, fmt.Errorf("snapshot name not found in the parameters")
	}

	// if it is active cluster, then send the backup state
	if mode == EngineModeActive {
		returnCode, err := retrieveBackupState(d.K8sClient, pluginParameters, snapshotName)
		if err != nil {
			d.Log.Error(err, "failed to retrieve backup state")
		}
		return returnCode, err
	} else {
		// if it is destination cluster, then send restore status
		returnCode, err := retrieveRestoreState(d.K8sClient, pluginParameters, snapshotName)
		if err != nil {
			d.Log.Error(err, "failed to retrieve restore state")
		}
		return returnCode, err
	}
}

// retrieveSnapshotState hits the Snapshot API of Elasticsearch to retrieve the backup status of a Snapshot
func retrieveBackupState(k8sClient kubernetes.Interface, params PluginParameters, snapshotName string) (int32, error) {
	// crate an Elasticsearch client
	esClient, err := NewElasticsearchClient(k8sClient, params.Elasticsearch)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed create new Elasticsearch client")
	}

	// configure snapshot get request
	snapshotGetRequest := esapi.SnapshotGetRequest{
		Repository: params.Repository.Name,
		Snapshot:   []string{snapshotName},
		Pretty:     true,
	}

	// get snapshot status
	fmt.Println("Requesting for snapshot status.....")
	resp, err := snapshotGetRequest.Do(context.Background(), esClient)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed to send SnapshotGetRequest")
	}
	defer resp.Body.Close()

	fmt.Println("Response: ", resp.String())

	if resp.StatusCode != http.StatusOK {
		rootCause, err := parseErrorCause(resp.Body)
		if err != nil {
			return framework.Errored, fmt.Errorf("failed to retrieve backup state.\n"+
				"Also, failed to parse the error info. Reason: %s", err.Error())
		}
		return framework.Errored, fmt.Errorf("failed to retrieve backup state.\n"+
			"Error Type: %s, Reason: %s", rootCause.Type, rootCause.Reason)
	}

	var statusResponse SnapshotGetResponse
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed to read SnapshotGetResponse")
	}

	if len(bytes.TrimSpace(data)) == 0 { // The source ES does not contain any data. So, nothing to backup.
		return framework.Completed, nil
	}

	err = json.Unmarshal(data, &statusResponse)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed to unmarshal SnapshotGetResponse")
	}

	for _, s := range statusResponse.Snapshots {
		if s.Snapshot == snapshotName {
			switch s.State {
			case SnapshotSucceeded:
				return framework.Completed, nil
			case SnapshotInProgress:
				return framework.InProgress, nil
			case SnapshotFailed:
				return framework.Failed, nil // TODO: return failure reason.
			default:
				return framework.Unknown, fmt.Errorf("snapshot status unknown")
			}
		}
	}
	return framework.Invalid, fmt.Errorf("no snapshot found with named %s", snapshotName)
}

// retrieveRestoreState hits the Recovery API of Elasticsearch to retrieve the restore status of a ES.
func retrieveRestoreState(k8sClient kubernetes.Interface, params PluginParameters, snapshotName string) (int32, error) {
	// crate an Elasticsearch client
	esClient, err := NewElasticsearchClient(k8sClient, params.Elasticsearch)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed to create Elasticsearch client")
	}

	// configure indexes recovery request
	indexRecoveryRequest := esapi.IndicesRecoveryRequest{
		Index:  nil,
		Pretty: true,
	}

	// get recovery status
	fmt.Println("Requesting for recovery status........")
	resp, err := indexRecoveryRequest.Do(context.Background(), esClient)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed to send IndexRecoveryRequest")
	}
	defer resp.Body.Close()

	fmt.Println("Response: ", resp.String())

	if resp.StatusCode != http.StatusOK {
		rootCause, err := parseErrorCause(resp.Body)
		if err != nil {
			return framework.Errored, fmt.Errorf("failed to retrieve restore state.\n"+
				"Also, failed to parse the error info. Reason: %s", err.Error())
		}
		return framework.Errored, fmt.Errorf("failed to retrieve restore state.\n"+
			"Error Type: %s, Reason: %s", rootCause.Type, rootCause.Reason)
	}

	// parse recovery response
	var recoveryStats map[string]IndexesRecoveryStatus
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed to read IndexRecoveryResponse")
	}

	if len(bytes.TrimSpace(data)) == 0 { // The source ES does not contain any data. So, nothing to recover.
		return framework.Completed, nil
	}

	err = json.Unmarshal(data, &recoveryStats)
	if err != nil {
		return framework.Errored, errors.Wrap(err, "failed to unmarshal IndexRecoveryResponse")
	}

	recoveryInitiated := false
	for _, index := range recoveryStats {
		recoveryInitiated = true
		for _, shard := range index.Shards {
			if shard.Type == RecoveryTypeSnapshot && shard.Source.Snapshot != snapshotName {
				// not recovering from the current snapshot. so, this is outside of current scope.
				continue
			}
			if shard.Stage != RecoveryStageDone {
				return framework.InProgress, nil
			}
		}
	}
	if !recoveryInitiated {
		return framework.InProgress, nil
	}
	return framework.Completed, nil
}
