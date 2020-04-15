# Elasticsearch Plugin Overview

This guide will give you an overview of how Elasticsearch data mobility works with Kubemove. Here, we are going to presents a diagram that shows the steps that happen in Kubemove to sync data between two Elasticsearches. Then, we will try to explain those steps.

## How Elasticsearch Sync Works

The following diagram shows the steps that happen to sync data between two Elasticsearches deployed in two different clusters.

![Elasticserch Sync Overview](/images/overview.jpg)

As you can see in the above diagram, we can divide the data mobility flow into the following phases:

1. **Preparation:** In this phase, the user prepares his clusters, Elasticsearches, Backend, Plugin, etc.
2. **Initialization:** In this phase Kubemove initializes the plugins.
3. **Sync:** In this phase, Kubemove sync data between the two Elasticsearches according to the sync schedule.

### Preparation

In the preparation phase, the user prepares his Elasticsearches. Then, he creates the necessary backend secret. Then, he creates `MovePair` CR to connect the two clusters. Finally, he creates a `MoveEngine` CR in the source cluster with the necessary information of the backend and the desired Elasticsearches.

### Initialization

When the user creates a `MoveEngine` CR, the initialization phase starts. In this phase, Kubemove initializes the Elasticsearch plugins of both clusters that ensure the plugins are ready to sync data on each schedule. The initialization part can be summarized in the following steps:

- 2.1. MoveEngine controller of the source cluster watches for the `MoveEngine` CR.
- 2.2. When it sees a `MoveEngine` CR, it creates another `MoveEngine` CR in the destination cluster with `standby` mode. Then, the following steps happen in the destination cluster:
    - 2.2.1. MoveEngine controller of the destination cluster watches for the `MoveEngine` CR.
    - 2.2.2. Then it invokes the `INIT` API of the plugin.
    - 2.2.3. The plugin patches the Elasticsearch CR to inject a snapshot repository plugin installer init-container. The Elasticsearch nodes restart, install the repository plugin and register the desired repository.
    - 2.2.4. The plugin notifies the MoveEngine controller that the plugin registration has completed.
    - 2.2.5. The MoveEngine controller updates the MoveEngine status to `Ready` to indicate that the plugin of the destination cluster is ready to sync data.
- 2.3. After creating a `MoveEngine` CR in the destination cluster, it invokes the `INIT` API of the plugin of the source cluster. Then, the following steps happen:
  - 2.3.1. The plugin patches the Elasticsearch CR to inject a snapshot repository plugin installer init-container. The Elasticsearch nodes restart, install the plugin and register the desired repository.
  - 2.3.2. The plugin notifies the MoveEngine controller that the plugin registration has completed.
  - 2.3.3. The MoveEngine controller updates the `MoveEngine` CR status to `Initialized` to indicate the source plugin is has been initialized successfully.
- 2.4. The MoveEngine controller waits for the standby `MoveEngine` CR status to be `Ready`.
- 2.5. When the standby `MoveEngine` CR is ready, the MoveEngine controller promotes the status of the active `MoveEngine` from `Initialized` to `Ready` to indicates that both of the plugins are now ready to start a sync.

### Sync

Once the plugins have been initialized, they become ready to handle the sync according to the sync schedule. When a sync schedule appears, the following things happen:

- 3.1. The MoveEngine controller of the source cluster creates a `DataSync` CR with `backup` mode to trigger a sync.
- 3.2. The DataSync controller of the source cluster watches for the `DataSync` CR.
- 3.3. When it sees a `DataSync` CR, it invokes the `SYNC` API of the source plugin to initiate a backup.
- 3.4. Then, the plugin invokes the snapshot API of the Elasticsearch. The source Elasticsearch takes a snapshot in the registered repository.
- 3.5. After invoking the `SYNC` API, the DataSync controller continuously invokes the `STATUS` API of the plugin to determine the backup status.
- 3.6. When the backup phase is completed, the DataSync controller sets the status of `DataSync` CR to `Completed`.
- 3.7. The MoveEngine controller of source cluster watches for the `DataSync` CR status.
- 3.8. When it sees the backup phase has completed, it creates a `DataSync` CR with `restore` mode in the destination cluster.
- 3.9. The DataSync controller of the destination cluster watches for the `DataSync` CR.
- 3.10. When it sees a `DataSync` CR, it invokes `SYNC` API of the plugin of the destination cluster to initiate a restore from the latest backup.
- 3.11. Then, the plugin invokes restore API of Elasticsearch to start restoring.
- 3.12. After invoking the `SYNC` API, the DataSync controller continuously invokes the `STATUS` API to determine the restore status.
- 3.13. Once the restore is completed, it sets the `DataSync` status to `Completed`.
- 3.14. The MoveEngine controller of the destination cluster watches for the `DataSync` CR.
- 3.15. When it sees the restore has completed, it set the `dataSyncStatus` field to `Completed` to indicate that the restore has been completed.
- 3.16. The MoveEngine controller of the source cluster watches for `MoveEngine` CR of destination cluster.
- 3.17. Finally, when the MoveEngine controller sees that the restore has completed in the destination cluster, it set the `dataSyncStatus` filed of `MoveEngine` CR of both source and destination clusters to `Synced` to denote successful sync.

We hope, now you have a basic understanding of how Elasticsearch data mobility works with Kubemove. Your next steps should be testing this process by following the user guide [here](/docs/user-guide.md).
