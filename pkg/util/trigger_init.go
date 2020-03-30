package util

import (
	"context"
	"fmt"

	"github.com/kubemove/elasticsearch-plugin/pkg/plugin"

	"github.com/kubemove/kubemove/pkg/plugin/proto"
	"google.golang.org/grpc"
)

// TriggerInit prepare source and destination Elasticsearch to backup and restore from a Minio repository
func (opt *PluginOptions) TriggerInit() error {
	fmt.Println("Establishing gRPC connection with ECK plugin of source cluster")
	srcConn, err := grpc.Dial(opt.SrcPluginAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer srcConn.Close()
	srcClient := proto.NewDataSyncerClient(srcConn)

	fmt.Println("Establishing gRPC connection with ECK plugin of destination cluster")
	dstConn, err := grpc.Dial(opt.DstPluginAddress, grpc.WithInsecure(), grpc.WithBlock())
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

	params := plugin.PluginParameters{
		Elasticsearch: plugin.ElasticsearchOptions{
			Name:               "sample-es",
			Namespace:          "default",
			ServiceName:        "sample-es-es-http",
			Scheme:             "https",
			Port:               9200,
			AuthSecret:         "sample-es-es-elastic-user",
			TLSSecret:          "sample-es-es-http-ca-internal",
			InsecureSkipVerify: true,
		},
	}
	err = plugin.WaitUntilElasticsearchReady(opt.SrcKubeClient, opt.SrcDmClient, params)
	if err != nil {
		return err
	}

	fmt.Println("Registering Repository in the destination Elasticsearch")
	initResp, err = dstClient.Init(context.Background(), initParams)
	if err != nil {
		return err
	}
	fmt.Println(initResp)

	return plugin.WaitUntilElasticsearchReady(opt.DstKubeClient, opt.DstDmClient, params)
}
