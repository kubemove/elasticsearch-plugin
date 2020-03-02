package main

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/spf13/cobra"

	"github.com/kubemove/kubemove/pkg/plugin/ddm/plugin"

	"github.com/kubemove/kubemove/pkg/plugin/proto"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/util/wait"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	common "github.com/elastic/cloud-on-k8s/pkg/apis/common/v1"
	eck "github.com/elastic/cloud-on-k8s/pkg/apis/elasticsearch/v1"
	esPlugin "github.com/kubemove/elasticsearch-plugin/pkg/plugin"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeConfigPath   string
	srcContext       string
	dstContext       string
	srcPluginAddress string
	dstPluginAddress string
	indexName        string
	indexFrom        string
	srcClusterIp     string
	dstClusterIp     string
	srcESNodePort    int32
	dstESNodePort    int32

	srcKubeClient kubernetes.Interface
	dstKubeClient kubernetes.Interface
	srcDmClient   dynamic.Interface
	dstDmClient   dynamic.Interface
)

const (
	SourceClusterIP              = "SRC_CLUSTER_IP"
	DestinationClusterIP         = "DST_CLUSTER_IP"
	SourceESServiceNodePort      = "SRC_ES_NODE_PORT"
	DestinationESServiceNodePort = "DST_ES_NODE_PORT"
	EngineMode                   = "ENGINE_MODE"
)

func main() {
	err := rootCmd().Execute()
	if err != nil {
		panic(err)
	}
}

func rootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "eck-plugin",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}

			srcConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{CurrentContext: srcContext}).ClientConfig()
			if err != nil {
				return err
			}
			dstConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{CurrentContext: dstContext}).ClientConfig()
			if err != nil {
				return err
			}

			srcKubeClient, err = kubernetes.NewForConfig(srcConfig)
			if err != nil {
				return err
			}
			srcDmClient, err = dynamic.NewForConfig(srcConfig)
			if err != nil {
				return err
			}

			dstKubeClient, err = kubernetes.NewForConfig(dstConfig)
			if err != nil {
				return err
			}
			dstDmClient, err = dynamic.NewForConfig(dstConfig)
			if err != nil {
				return err
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfigpath", "", "KubeConfig path")
	rootCmd.PersistentFlags().StringVar(&srcContext, "src-context", "", "Source Context")
	rootCmd.PersistentFlags().StringVar(&dstContext, "dst-context", "", "Destination Context")
	rootCmd.PersistentFlags().StringVar(&srcPluginAddress, "src-plugin", "", "URL of the source plugin")
	rootCmd.PersistentFlags().StringVar(&dstPluginAddress, "dst-plugin", "", "URL of the destination plugin")
	rootCmd.PersistentFlags().StringVar(&indexName, "index-name", "test-index", "Name of the index to insert")
	rootCmd.PersistentFlags().StringVar(&indexFrom, "index-from", "active", "Mode of targeted es")
	rootCmd.PersistentFlags().StringVar(&srcClusterIp, "src-cluster-ip", "", "IP address of the source cluster")
	rootCmd.PersistentFlags().StringVar(&dstClusterIp, "dst-cluster-ip", "", "IP address of the source cluster")
	rootCmd.PersistentFlags().Int32Var(&srcESNodePort, "src-es-nodeport", 0, "Node port of source ES service")
	rootCmd.PersistentFlags().Int32Var(&dstESNodePort, "dst-es-nodeport", 0, "Node port of source ES service")

	rootCmd.AddCommand(triggerInit())
	rootCmd.AddCommand(triggerSync())
	rootCmd.AddCommand(insertIndexes())
	rootCmd.AddCommand(showIndexes())

	return rootCmd
}

func triggerInit() *cobra.Command {
	cmd := &cobra.Command{
		Use: "trigger-init",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := setup()
			if err != nil {
				return err
			}

			fmt.Println("Installing Minio Repository plugin in the source ES")
			err = insertMinioRepository(srcDmClient)
			if err != nil {
				return err
			}

			fmt.Println("Installing Minio Repository plugin in the destination ES")
			err = insertMinioRepository(dstDmClient)
			if err != nil {
				return err
			}

			fmt.Println("Establishing gRPC connection with ECK plugin of source cluster")
			srcConn, err := grpc.Dial(srcPluginAddress, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				return err
			}
			defer srcConn.Close()
			srcClient := proto.NewDataSyncerClient(srcConn)

			fmt.Println("Establishing gRPC connection with ECK plugin of destination cluster")
			dstConn, err := grpc.Dial(dstPluginAddress, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				return err
			}
			defer dstConn.Close()
			dstClient := proto.NewDataSyncerClient(dstConn)

			initParams := &proto.InitRequest{
				Params: map[string]string{
					"engineName":      "sample-es-move",
					"engineNamespace": "default",
				},
			}

			fmt.Println("Registering Repository in the source Elasticsearch")
			initResp, err := srcClient.Init(context.Background(), initParams)
			if err != nil {
				return err
			}
			fmt.Println(initResp.String())

			fmt.Println("Registering Repository in the destination Elasticsearch")
			initResp, err = dstClient.Init(context.Background(), initParams)
			if err != nil {
				return err
			}
			fmt.Println(initResp)

			return nil
		},
	}

	return cmd
}

func triggerSync() *cobra.Command {
	cmd := &cobra.Command{
		Use: "trigger-sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Establishing gRPC connection with ECK plugin of source cluster")
			srcConn, err := grpc.Dial(srcPluginAddress, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				return err
			}
			defer srcConn.Close()
			srcClient := proto.NewDataSyncerClient(srcConn)

			fmt.Println("Establishing gRPC connection with ECK plugin of destination cluster")
			dstConn, err := grpc.Dial(dstPluginAddress, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				return err
			}
			defer dstConn.Close()
			dstClient := proto.NewDataSyncerClient(dstConn)

			snapshotName := fmt.Sprintf("snapshot-%d", time.Now().Unix())
			syncParams := &proto.SyncRequest{
				Params: map[string]string{
					"engineName":      "sample-es-move",
					"engineNamespace": "default",
					"snapshotName":    snapshotName,
				},
			}

			fmt.Println("Triggering Backup in the source Elasticsearch")
			syncResp, err := srcClient.SyncData(context.Background(), syncParams)
			if err != nil {
				return err
			}
			fmt.Println("Response: ", syncResp.String())

			statusParams := &proto.SyncStatusRequest{
				Params: map[string]string{
					"engineName":      "sample-es-move",
					"engineNamespace": "default",
					"snapshotName":    snapshotName,
				},
			}

			fmt.Println("Waiting for Snapshot to be completed")
			err = wait.PollImmediate(2*time.Second, 5*time.Minute, func() (done bool, err error) {
				statusResp, err := srcClient.SyncStatus(context.Background(), statusParams)
				if statusResp == nil {
					return true, err
				}
				switch statusResp.Status {
				case plugin.Completed:
					return true, nil // snapshot completed successfully. we are done.
				case plugin.InProgress:
					return false, nil // snapshot is running. so, retry.
				default:
					return true, err // snapshot process has encountered a failure. so, no need to retry.
				}
			})
			if err != nil {
				return err
			}

			fmt.Println("Triggering Restore in the destination Elasticsearch")
			syncResp, err = dstClient.SyncData(context.Background(), syncParams)
			if err != nil {
				return err
			}
			fmt.Println("Response: ", syncResp.String())

			fmt.Println("Waiting for Restore to be completed")
			err = wait.PollImmediate(2*time.Second, 5*time.Minute, func() (done bool, err error) {
				statusResp, err := dstClient.SyncStatus(context.Background(), statusParams)
				if statusResp == nil {
					return true, err
				}
				switch statusResp.Status {
				case plugin.Completed:
					return true, nil // restore completed successfully. we are done.
				case plugin.InProgress:
					return false, nil // restore is running. so, retry.
				default:
					return true, err // restore process has encountered a failure. so, no need to retry.
				}
			})
			return err
		},
	}

	return cmd
}

func insertIndexes() *cobra.Command {
	cmd := &cobra.Command{
		Use: "insert-index",
		RunE: func(cmd *cobra.Command, args []string) error {

			client, err := getESClient(srcKubeClient, srcClusterIp, srcESNodePort)
			if err != nil {
				return err
			}

			req := esapi.IndicesCreateRequest{
				Index:           indexName,
				Body:            nil,
				IncludeTypeName: nil,
				Pretty:          true,
			}

			resp, err := req.Do(context.Background(), client)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			fmt.Println(resp.String())
			return nil
		},
	}
	return cmd
}

func showIndexes() *cobra.Command {
	cmd := &cobra.Command{
		Use: "show-indexes",
		RunE: func(cmd *cobra.Command, args []string) error {
			var address string
			var port int32
			var kubeClient kubernetes.Interface
			var err error

			if indexFrom == esPlugin.EngineModeActive {
				port = srcESNodePort
				address = srcClusterIp
				kubeClient = srcKubeClient
			} else {
				port = dstESNodePort
				address = dstClusterIp
				kubeClient = dstKubeClient
			}

			client, err := getESClient(kubeClient, address, port)
			if err != nil {
				return err
			}

			req := esapi.IndicesGetRequest{
				Index:  []string{"_all"},
				Pretty: true,
			}

			resp, err := req.Do(context.Background(), client)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			fmt.Println(resp.String())
			return nil

		},
	}

	return cmd
}

func insertMinioRepository(dmClient dynamic.Interface) error {
	gvr := schema.GroupVersionResource{
		Group:    "elasticsearch.k8s.elastic.co",
		Version:  "v1",
		Resource: "elasticsearches",
	}

	es, err := getElasticsearch(dmClient, gvr)
	if err != nil {
		return err
	}

	// insert plugin installer init-container
	es.Spec.NodeSets[0].PodTemplate = corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: "install-plugins",
					Command: []string{
						"sh",
						"-c",
						"bin/elasticsearch-plugin install --batch repository-s3",
					},
				},
			},
		},
	}

	// insert minio credentiaals
	es.Spec.SecureSettings = []common.SecretSource{
		{
			SecretName: "minio-credentials",
		},
	}

	// convert updated Elasticsearch back to unstructured object
	updatedES, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&es)
	if err != nil {
		fmt.Println("Error during converting into unstructured object")
		return err
	}

	// update Elasticsearch
	_, err = dmClient.Resource(gvr).Namespace("default").Update(&unstructured.Unstructured{Object: updatedES}, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// give a delay for the ES phase to be updated
	time.Sleep(5 * time.Second)

	// wait for ES to be ready with the plugin installer
	err = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		es, err := getElasticsearch(dmClient, gvr)
		if err == nil {
			return es.Status.Phase == eck.ElasticsearchReadyPhase, nil
		}
		if !kerr.IsNotFound(err) {
			return true, err
		}
		return false, nil
	})
	return err
}

func getElasticsearch(dmClient dynamic.Interface, gvr schema.GroupVersionResource) (*eck.Elasticsearch, error) {
	// read Elasticsearch object
	resp, err := dmClient.Resource(gvr).Namespace("default").Get("sample-es", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// convert to unstructured object into Elasticsearch type
	var es eck.Elasticsearch
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(resp.UnstructuredContent(), &es)
	if err != nil {
		return nil, err
	}
	return &es, nil
}

func setup() error {
	loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}

	srcConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{CurrentContext: srcContext}).ClientConfig()
	if err != nil {
		return err
	}
	dstConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{CurrentContext: dstContext}).ClientConfig()
	if err != nil {
		return err
	}

	srcKubeClient, err = kubernetes.NewForConfig(srcConfig)
	if err != nil {
		return err
	}
	srcDmClient, err = dynamic.NewForConfig(srcConfig)
	if err != nil {
		return err
	}

	dstKubeClient, err = kubernetes.NewForConfig(dstConfig)
	if err != nil {
		return err
	}
	dstDmClient, err = dynamic.NewForConfig(dstConfig)
	if err != nil {
		return err
	}
	return nil
}

func getESClient(kubeClient kubernetes.Interface, address string, port int32) (*elasticsearch.Client, error) {
	return esPlugin.NewElasticsearchClient(kubeClient, esPlugin.ElasticsearchOptions{
		ServiceName:        address,
		Namespace:          "default",
		Scheme:             "https",
		Port:               port,
		AuthSecret:         "sample-es-es-elastic-user",
		TLSSecret:          "sample-es-es-http-ca-internal",
		InsecureSkipVerify: true,
	})
}
