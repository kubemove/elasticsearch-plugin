package util

import (
	"context"
	"fmt"
	"time"

	"github.com/kubemove/kubemove/pkg/plugin/ddm/plugin"

	"github.com/kubemove/kubemove/pkg/plugin/proto"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/util/wait"
)

// TriggerSync calls SYNC API of the plugin in source cluster to initiate backup.
// Then, it waits for the backup to be completed. It continuously calls STATUS API to to check backup state.
// Once the backup is completed, it calls SYNC API of the plugin in destination cluster which initiate a restore.
// Then, it's wait for restore to be completed. Again, it uses STATUS API to check the restore sate.
func (opt *PluginOptions) TriggerSync() error {
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
}
