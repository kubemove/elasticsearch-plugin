package main

import (
	"fmt"

	"github.com/appscode/go/flags"
	"github.com/kubemove/elasticsearch-plugin/pkg/util"
	"github.com/spf13/cobra"

	"github.com/kubemove/elasticsearch-plugin/pkg/plugin"
	framework "github.com/kubemove/kubemove/pkg/plugin/ddm/plugin"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	PluginVersion = "v7.x"
)

var opt util.PluginOptions

func main() {
	err := rootCmd().Execute()
	if err != nil {
		fmt.Printf("failed to execute command. Reason: %v", err.Error())
	}
	utilruntime.Must(err)
}

func rootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "elasticsearch-plugin",
		Short: "Plugin to move Elasticsearch data",
		Long:  "Kubemove plugin for Elasticsearch deployed with ECK operator",
	}
	rootCmd.AddCommand(cmdStart())
	rootCmd.AddCommand(cmdRun())

	return rootCmd
}

func cmdStart() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start Elasticsearch Plugin",
		RunE: func(cmd *cobra.Command, args []string) error {
			var log = logf.Log.WithName(fmt.Sprintf("ECK Plugin %s", PluginVersion))

			config, err := rest.InClusterConfig()
			if err != nil {
				log.Error(err, "failed to build kubernetes client")
				utilruntime.Must(err)
			}

			return framework.Register("elasticsearch-plugin",
				&plugin.ElasticsearchDDM{
					Log:       log,
					K8sClient: kubernetes.NewForConfigOrDie(config),
					DmClient:  dynamic.NewForConfigOrDie(config),
				})
		},
	}
}

func cmdRun() *cobra.Command {
	cmdRun := &cobra.Command{
		Use:          "run",
		Short:        "Run some helper commands to test the plugin",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if opt.Debug {
				flags.DumpAll(cmd.Flags())
			}
			err := opt.Setup()
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmdRun.PersistentFlags().StringVar(&opt.KubeConfigPath, "kubeconfig", "", "KubeConfig path")
	cmdRun.PersistentFlags().StringVar(&opt.SrcContext, "src-context", "", "Source Context")
	cmdRun.PersistentFlags().StringVar(&opt.DstContext, "dst-context", "", "Destination Context")
	cmdRun.PersistentFlags().StringVar(&opt.SrcPluginAddress, "src-plugin", "", "URL of the source plugin")
	cmdRun.PersistentFlags().StringVar(&opt.DstPluginAddress, "dst-plugin", "", "URL of the destination plugin")
	cmdRun.PersistentFlags().StringVar(&opt.EsName, "es-name", "sample-es", "Name of the Elasticsearch")
	cmdRun.PersistentFlags().StringVar(&opt.EsNamespace, "es-namespace", "default", "Namespace of the Elasticsearch")
	cmdRun.PersistentFlags().StringVar(&opt.IndexName, "index-name", "test-index", "Name of the index to insert")
	cmdRun.PersistentFlags().StringVar(&opt.IndexFrom, "index-from", "active", "Mode of targeted es")
	cmdRun.PersistentFlags().StringVar(&opt.SrcClusterIp, "src-cluster-ip", "", "IP address of the source cluster")
	cmdRun.PersistentFlags().StringVar(&opt.DstClusterIp, "dst-cluster-ip", "", "IP address of the source cluster")
	cmdRun.PersistentFlags().Int64Var(&opt.SrcESNodePort, "src-es-nodeport", 0, "Node port of source ES service")
	cmdRun.PersistentFlags().Int64Var(&opt.DstESNodePort, "dst-es-nodeport", 0, "Node port of source ES service")
	cmdRun.PersistentFlags().BoolVar(&opt.Debug, "debug", false, "Specify whether to print debug info")

	cmdRun.AddCommand(cmdTriggerInit())
	cmdRun.AddCommand(cmdTriggerSync())
	cmdRun.AddCommand(cmdInsertIndexes())
	cmdRun.AddCommand(cmdShowIndexes())

	return cmdRun
}

func cmdTriggerInit() *cobra.Command {
	return &cobra.Command{
		Use:          "trigger-init",
		Short:        "Prepare Elasticsearches for sync",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opt.TriggerInit()
		},
	}
}

func cmdTriggerSync() *cobra.Command {
	return &cobra.Command{
		Use:          "trigger-sync",
		Short:        "Trigger SYNC API of the plugin for backup and restore",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opt.TriggerSync()
		},
	}
}

func cmdInsertIndexes() *cobra.Command {
	return &cobra.Command{
		Use:          "insert-index",
		Short:        "Insert an index into a Elasticsearch",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opt.InsertIndex()
		},
	}
}

func cmdShowIndexes() *cobra.Command {
	return &cobra.Command{
		Use:          "show-indexes",
		Short:        "Show all indexes of a Elasticsearch",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := opt.ShowIndexes()
			if err != nil {
				return err
			}
			fmt.Println(resp)
			return nil
		},
	}
}
