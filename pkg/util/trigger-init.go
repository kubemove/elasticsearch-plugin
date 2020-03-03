package util

import (
	"context"
	"fmt"

	"github.com/kubemove/kubemove/pkg/plugin/proto"
	"google.golang.org/grpc"
)

// TriggerInit prepare source and destination Elasticsearch to backup and restore from a Minio repository
func (opt *PluginOptions) TriggerInit() error {
	fmt.Println("Installing Minio Repository plugin in the source ES")
	err := insertMinioRepository(opt.SrcDmClient)
	if err != nil {
		return err
	}

	fmt.Println("Installing Minio Repository plugin in the destination ES")
	err = insertMinioRepository(opt.DstDmClient)
	if err != nil {
		return err
	}

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

	fmt.Println("Registering Repository in the destination Elasticsearch")
	initResp, err = dstClient.Init(context.Background(), initParams)
	if err != nil {
		return err
	}
	fmt.Println(initResp)

	return nil
}
