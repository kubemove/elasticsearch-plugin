package util

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v7/esapi"
)

// InsertIndex insert a index in the source Elasticsearch
func (opt *PluginOptions) InsertIndex() error {
	client, err := getESClient(opt.SrcKubeClient, opt.SrcClusterIp, int32(opt.SrcESNodePort))
	if err != nil {
		return err
	}

	req := esapi.IndicesCreateRequest{
		Index:           opt.IndexName,
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
}
