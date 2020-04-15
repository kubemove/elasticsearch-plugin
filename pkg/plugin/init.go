package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"

	"github.com/elastic/go-elasticsearch/v7/esapi"
)

type RepoCreateRequestBody struct {
	Type     string             `json:"type"`
	Settings RepositorySettings `json:"settings"`
}

type RepositorySettings struct {
	Bucket   string `json:"bucket"`
	BasePath string `json:"base_path,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	ReadOnly string `json:"read_only,omitempty"`
}

func (d *ElasticsearchDDM) Init(params map[string]string) error {
	fmt.Println("\nHandling INIT request..................")
	// extract the plugin parameters from the respective MoveEngine
	pluginParameters, mode, err := extractPluginParameters(d.DmClient, params)
	if err != nil {
		d.Log.Error(err, "failed to extract plugin parameters")
		return err
	}

	// inject s3-repository plugin installer in the Elasticsearch
	err = insertRepositoryPluginInstaller(d.K8sClient, d.DmClient, pluginParameters)
	if err != nil {
		d.Log.Error(err, "failed to insert repository plugin-installer")
		return err
	}

	// Register Snapshot Repository in the Elasticsearch
	err = registerSnapshotRepository(d.K8sClient, pluginParameters, mode)
	if err != nil {
		d.Log.Error(err, "failed to register repository")
	}
	return err
}

// registerSnapshotRepository hits Snapshot API of Elasticsearch to register a repository
func registerSnapshotRepository(k8sClient kubernetes.Interface, params PluginParameters, mode string) error {
	// crate an Elasticsearch client
	esClient, err := NewElasticsearchClient(k8sClient, params.Elasticsearch)
	if err != nil {
		return errors.Wrap(err, "failed to get Elasticsearch client")
	}

	// configure request body
	body := RepoCreateRequestBody{
		Type: params.Repository.Type,
		Settings: RepositorySettings{
			Bucket: params.Repository.Bucket,
		},
	}
	if params.Repository.Prefix != "" {
		body.Settings.BasePath = params.Repository.Prefix
	}
	if params.Repository.Endpoint != "" {
		body.Settings.Endpoint = params.Repository.Endpoint
	}
	if params.Repository.Scheme != "" {
		body.Settings.Protocol = params.Repository.Scheme
	}
	if mode != EngineModeActive {
		body.Settings.ReadOnly = "true"
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return errors.Wrap(err, "failed to marshal RepoCreateRequestBody")
	}

	// configure snapshot repository create request
	repoRequest := esapi.SnapshotCreateRepositoryRequest{
		Repository: params.Repository.Name,
		Body:       bytes.NewReader(jsonBody),
		Pretty:     true,
	}

	fmt.Println("Registering repository..................")
	// register repository
	resp, err := repoRequest.Do(context.Background(), esClient)
	if err != nil {
		return errors.Wrap(err, "failed to send SnapshotCreateRepositoryRequest")
	}
	defer resp.Body.Close()

	fmt.Println("Response: ", resp.String())

	if resp.StatusCode != http.StatusOK {
		rootCause, err := parseErrorCause(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to register Snapshot Repository to the Elasticsearch.\n"+
				"Also, failed to parse the error info. Reason: %s", err.Error())
		}
		return fmt.Errorf("failed to register Snapshot Repository to the Elasticsearch.\n"+
			"Error Type: %s, Reason: %s", rootCause.Type, rootCause.Reason)
	}
	return nil
}
