# Sync Data Between two Elasticsearch

This guide will give you a step by step instructions on how to sync data between two Elasticsearch deployed in two different clusters using the Elasticsearch official [ECK](https://github.com/elastic/cloud-on-k8s) operator.

## Pre-requisites

- At first, you will need two Kubernetes clusters. Here, we are going to use two local clusters created using [kind](https://github.com/kubernetes-sigs/kind). You can use any cluster of your choice.
- Then, you will require a cloud bucket. Here, we are going to use a Minio server for this purpose. You can use any cloud bucket supported by Elasticsearch as listed [here](https://www.elastic.co/guide/en/elasticsearch/reference/current/snapshots-register-repository.html#snapshots-repository-plugins).
- Install the official Elasticsearch operator for Kubernetes by following the instructions from [here](https://www.elastic.co/downloads/elastic-cloud-kubernetes).
- Install Kubemove in both clusters by following the instructions from [here](https://github.com/kubemove/kubemove#deploy-kubemove).
- Install the Elasticsearch plugin for Kubemove in both clusters by following the instructions from [here](https://github.com/kubemove/elasticsearch-plugin/tree/master#install-plugin).
- If you are not familiar with how Elasticsearch data mobility works with Kubemove, please read the overview guide from [here](/docs/overview.md).

>If you are using two local kind clusters, you have to increase the maximum map count for VM. Otherwise, the Elasticsearch might not get into the `Ready` phase. In Linux, you can set it using `sudo sysctl -w vm.max_map_count=262144`.

## Prepare Elasticsearch

In this section, we are going to deploy one Elasticsearch in the source cluster and another Elasticsearch in the destination cluster. Then, we will insert some sample data in the source Elasticsearch.

**Deploy Elasticsearch:**

Here, is the YAML of the sample Elasticsearch we are going to deploy,

```yaml
apiVersion: elasticsearch.k8s.elastic.co/v1
kind: Elasticsearch
metadata:
  name: sample-es
spec:
  version: 7.6.2
  nodeSets:
  - name: default
    count: 2
```

Let's deploy the above Elasticsearch into the source cluster,

```console
kubectl apply -f ./examples/user-guide/elasticsearch.yaml --context=<src cluster context>
```

Now, deploy the Elasticsearch into the destination cluster,

```console
kubectl apply -f ./examples/user-guide/elasticsearch.yaml --context=<dst cluster context>
```

Now, wait for the Elasticsearch of the source cluster to go into the `Ready` state.

```console
$ kubectl get elasticsearch --context=<src cluster context> -w
NAME        HEALTH    NODES   VERSION   PHASE             AGE
sample-es   unknown           7.6.2     ApplyingChanges   3s
sample-es   unknown           7.6.2     ApplyingChanges   51s
sample-es   unknown           7.6.2     ApplyingChanges   59s
sample-es   green     2       7.6.2     Ready             61s
```

Also, wait for the Elasticsearch of destination cluster to go into the `Ready` state.

```console
$ kubectl get elasticsearch --context=<dst cluster context> -w
NAME        HEALTH    NODES   VERSION   PHASE             AGE
sample-es   unknown           7.6.2     ApplyingChanges   11s
sample-es   unknown           7.6.2     ApplyingChanges   69s
sample-es   unknown           7.6.2     ApplyingChanges   70s
sample-es   green     2       7.6.2     Ready             72s
```

**Connection Information:**

Now, let's get the necessary information to connect with Elasticsearch.

Connection information for the source Elasticsearch,

- Service:

  ```console
  $ kubectl get service --context=<src cluster context> | grep http
  sample-es-es-http      ClusterIP   10.96.76.231   <none>        9200/TCP         4m3s
  ```

- Password:

  ```console
  $ kubectl get secret sample-es-es-elastic-user -o=jsonpath='{.data.elastic}' --context=<src cluster context> | base64 --decode
  dqt7pgh8xzsgx2csb4dg5t8m
  ```

Connection information for the destination Elasticsearch:

- Service:

  ```console
  $ kubectl get service --context=<dst cluster context> | grep http
  sample-es-es-http      ClusterIP   10.96.126.127   <none>        9200/TCP         4m36s
  ```

- Password:

  ```console
  $ kubectl get secret sample-es-es-elastic-user -o=jsonpath='{.data.elastic}' --context=<dst cluster context> | base64 --decode
  hgmh699lvljl5765w2jmdnl7
  ```

**Make the Elasticsearches accessible to the local machine:**

We will interact with the Elasticsearches with the `curl` CLI tool. So, we need to make them accessible from our local machine. We are going to port-forward their services to connect from the local machine.

- Port-forward source Elasticsearch:

  ```console
  $ kubectl port-forward service/sample-es-es-http 9200 --context=<src cluster context>
  Forwarding from 127.0.0.1:9200 -> 9200
  Forwarding from [::1]:9200 -> 9200
  ```

- Port-forward destination Elasticsearch:

  ```console
  $ kubectl port-forward service/sample-es-es-http 9201:9200 --context=<dst cluster context>
  Forwarding from 127.0.0.1:9201 -> 9200
  Forwarding from [::1]:9201 -> 9200
  ```

Here, we have port-forwarded source Elasticsearch into `9200` port and destination Elasticsearch into `9201` port of our local machine. They are now accessible though `localhost:9200` and `localhost:9201` addresses respectively.

**Insert sample data:**

Now, let's create a sample index named `first_index` in the source cluster,

```console
$ curl -u "elastic:dqt7pgh8xzsgx2csb4dg5t8m" -k -X PUT "https://localhost:9200/first_index?pretty"
{
  "acknowledged" : true,
  "shards_acknowledged" : true,
  "index" : "first_index"
}
```

Verify that the index has been created,

```console
$ curl -u "elastic:dqt7pgh8xzsgx2csb4dg5t8m" -k -X GET "https://localhost:9200/first_index?pretty"
{
  "first_index" : {
    "aliases" : { },
    "mappings" : { },
    "settings" : {
      "index" : {
        "creation_date" : "1586691305396",
        "number_of_shards" : "1",
        "number_of_replicas" : "1",
        "uuid" : "oDaBUG9fQKadad0W-wbhWQ",
        "version" : {
          "created" : "7060299"
        },
        "provided_name" : "first_index"
      }
    }
  }
}
```

Verify that the index is not present in the destination Elasticsearch,

```console
$ curl -u "elastic:hgmh699lvljl5765w2jmdnl7" -k -X GET "https://localhost:9201/first_index?pretty"
{
  "error" : {
    "root_cause" : [
      {
        "type" : "index_not_found_exception",
        "reason" : "no such index [first_index]",
        "index_uuid" : "_na_",
        "resource.type" : "index_or_alias",
        "resource.id" : "first_index",
        "index" : "first_index"
      }
    ],
    "type" : "index_not_found_exception",
    "reason" : "no such index [first_index]",
    "index_uuid" : "_na_",
    "resource.type" : "index_or_alias",
    "resource.id" : "first_index",
    "index" : "first_index"
  },
  "status" : 404
}
```

Now, we are ready to schedule sync between the two Elasticsearches.

## Configure DatSync

In this section, we are going to schedule sync between the source and destination Elasticsearch.

**Create MovePair:**

At first, we need to connect the two clusters. Let's create a `MovePair` CR with the necessary connection information.

Below is the `MovePair` CR we are going to create. You must replace the relevant fields with your cluster information.

```yaml
apiVersion: kubemove.io/v1alpha1
kind: MovePair
metadata:
  name: local
  namespace: kubemove
spec:
  config:
    clusters:
    - cluster:
        certificate-authority-data: LS0tLS1CRUdJT....UNBVEUtLS0tLQo=
        server: https://172.17.0.2:6443
      name: kind-src-cluster
    - cluster:
        certificate-authority-data: LS0tLS1CRUd....LS0tLQo=
        server: https://172.17.0.3:6443
      name: kind-dst-cluster
    contexts:
    - context:
        cluster: kind-src-cluster
        user: kind-src-cluster
      name: kind-src-cluster
    - context:
        cluster: kind-dst-cluster
        user: kind-dst-cluster
      name: kind-dst-cluster
    current-context: kind-dst-cluster
    preferences: {}
    users:
    - name: kind-src-cluster
      user:
        client-certificate-data: LS0tLS1CRUdJTiBDRV....VElGSUNBVEUtLS0tLQo=
        client-key-data: LS0tLS1CRUdJTiBSU....kFURSBLRVktLS0tLQo=
    - name: kind-dst-cluster
      user:
        client-certificate-data: LS0tLS1CRUdJTiB....SVElGSUNBVEUtLS0tLQo=
        client-key-data: LS0tLS1CRUdJTiBSU....RSBLRVktLS0tLQo=
```

Let's create the `MovePair` CR.

```console
kubectl apply -f ./examples/user-guide/movepair.yaml --context=<src cluster context>
```

Verify that the two clusters are connected,

```console
$ kubectl get movepair -n kubemove local -o=jsonpath='{.status.state}' --context=<src cluster context>
Connected
```

**Create Backend Secret:**

Kubemove requires a cloud bucket to perform the sync operation. Here, we are going to use a Minio server as a repository. Let's create the necessary secret with the access credentials to our Minio repository,

Create Minio secret in the source cluster,

```console
kubectl create secret generic minio-credentials --context=<src cluster context> \
  --from-literal=s3.client.default.access_key=not@accesskey                     \
  --from-literal=s3.client.default.secret_key=not@secretkey
```

Create Minio secret in the destination cluster,

```console
kubectl create secret generic minio-credentials --context=<dst cluster context> \
  --from-literal=s3.client.default.access_key=not@accesskey                     \
  --from-literal=s3.client.default.secret_key=not@secretkey
```

>If you want to use other Elasticsearch supported repository, please modify the secret according to your setup.

If you want to use Minio server as a repository, you can easily deploy one in the destination cluster using the following command,

```console
kubectl apply -f ./examples/user-guide/minio_server.yaml --context=<dst cluster context>
```

**Create Bucket:**

Kubemove does not create a new bucket. So, the bucket we will use to store the data must be created manually. If you have deployed a Minio server by following the instruction above, it will create a NodePort service for the Minio server. You can create a bucket from Minio Web UI from `<your cluster ip>:<minio nodeport service>` url.

```console
$ kubectl get service minio --context=<dst cluster context>
NAME    TYPE       CLUSTER-IP     EXTERNAL-IP   PORT(S)          AGE
minio   NodePort   10.96.19.185   <none>        9000:32744/TCP   2m6s
```

**Create MoveEngine:**

Now, we have to create a `MoveEngine` CR with the repository and targeted Elasticsearch information to schedule sync. Below, is the YAML of the `MoveEngine` CR we are going to create for our `sample-es` Elasticsearch,

```yaml
apiVersion: kubemove.io/v1alpha1
kind: MoveEngine
metadata:
  name: sample-es-move
  namespace: kubemove
spec:
  movePair: local
  namespace: kubemove
  remoteNamespace: kubemove
  syncPeriod: "*/3 * * * *"
  plugin: elasticsearch-plugin
  includeResources: false
  mode: active
  pluginParameters:
    repository:
      name: minio_repo
      type: s3
      bucket: demo # replace with your bucket name
      prefix: es/backup
      endpoint: 172.17.0.3:32744 # minio server address. don't use `http://` or `https://` prefix.
      scheme: http
      credentials: minio-credentials
    elasticsearch:
      name: sample-es
      namespace: default
      serviceName: sample-es-es-http
      port: 9200
      scheme: https
      authSecret: sample-es-es-elastic-user
      tlsSecret: sample-es-es-http-ca-internal
```

Let's create the above `MoveEngine` CR in the source cluster,

```console
kubectl apply -f ./examples/user-guide/moveengine.yaml --context=<src cluster context>
```

Kubemove will automatically create a similar `MoveEngine` CR with `standby` mode in the destination cluster.

Verify that the standby `MoveEngine` has been created,

```console
$ kubectl get moveengine -n kubemove --context=<dst cluster context>
NAME             MODE      SYNC-PERIOD   STATUS         DATA-SYNC   SYNC-STATUS   SYNCED-TIME
sample-es-move   standby   */3 * * * *   Initializing
```

Now, Kubemove will patch the Elasticsearch CR and inject a plugin installer init-container. Once, the Elasticsearches are ready with repository plugin, Kubemove will register the repository. It may take a few minutes to complete the whole process.

Let's wait for the `MoveEngine` status to go into the `Ready` state.

```console
$ kubectl get moveengine -n kubemove --context=<src cluster context> -w
NAME             MODE     SYNC-PERIOD   STATUS         DATA-SYNC   SYNC-STATUS   SYNCED-TIME
sample-es-move   active   */3 * * * *   Initializing
sample-es-move   active   */3 * * * *   Initialized
sample-es-move   active   */3 * * * *   Ready
```

## Verify Sync

If you have come this far without any trouble, then you have successfully scheduled sync between your two Elasticsearches. Now, its time to verify whether the data are syncing or not.

On sync schedule, Kubemove will create a `DataSync`  CR with `backup` mode in the source cluster to trigger a backup and another `DataSync` CR with `restore` mode to trigger a restore from the backup.

Let's verify that a `DataSync` CR has been created in the source cluster,

```console
$ kubectl get datasync --all-namespaces --context=<src cluster context>
NAMESPACE   NAME                               MOVEENGINE       MODE     STATUS      COMPLETION-TIME
kubemove    ds-sample-es-move-20200412115400   sample-es-move   backup   Completed   2m24s
```

Also, verify that a `DataSync` CR has been created in  the destination cluster,

```console
$ kubectl get datasync --all-namespaces --context=<dst cluster context>
NAMESPACE   NAME                               MOVEENGINE       MODE      STATUS      COMPLETION-TIME
kubemove    ds-sample-es-move-20200412115400   sample-es-move   restore   Completed   3m5s
```

Once, the backup and restore phase of sync is completed, the `SYNC-STATUS` field of the `MoveEngine` will be `Synced`.

```console
$ kubectl get moveengine -n kubemove --context=<src cluster context> -w
NAME             MODE     SYNC-PERIOD   STATUS   DATA-SYNC                          SYNC-STATUS   SYNCED-TIME
sample-es-move   active   */3 * * * *   Ready    ds-sample-es-move-20200412115400   Running       2m39s
sample-es-move   active   */3 * * * *   Ready    ds-sample-es-move-20200412115400   Completed     2m49s
sample-es-move   active   */3 * * * *   Ready    ds-sample-es-move-20200412115400   Synced        0s
```

So, we can see from above that synced has been completed successfully. Now, its time to verify the synced data.

**Verify Synced Data:**

Let's check if the index `first_index` that we had created in the source Elasticsearch has been synced in the destination Elasticsearch,

```console
$ curl -u "elastic:hgmh699lvljl5765w2jmdnl7" -k -X GET "https://localhost:9201/_all?pretty"
{
  "first_index" : {
    "aliases" : { },
    "mappings" : { },
    "settings" : {
      "index" : {
        "creation_date" : "1586691305396",
        "number_of_shards" : "1",
        "number_of_replicas" : "1",
        "uuid" : "9K8EkSTVS1Wz_YoGsI1-mQ",
        "version" : {
          "created" : "7060299"
        },
        "provided_name" : "first_index"
      }
    }
  }
}
```

So, we can see from above that the sample data has been synced successfully. Now, let's create another index in the source Elasticsearch.

```console
$ curl -u "elastic:dqt7pgh8xzsgx2csb4dg5t8m" -k -X PUT "https://localhost:9200/second_index?pretty"
{
  "acknowledged" : true,
  "shards_acknowledged" : true,
  "index" : "second_index"
}
```

Now, wait for the next sync to complete. Then, check if the new data has been synced to the destination Elasticsearch,

```console
$ curl -u "elastic:hgmh699lvljl5765w2jmdnl7" -k -X GET "https://localhost:9201/_all?pretty"
{
  "first_index" : {
    "aliases" : { },
    "mappings" : { },
    "settings" : {
      "index" : {
        "creation_date" : "1586691305396",
        "number_of_shards" : "1",
        "number_of_replicas" : "1",
        "uuid" : "9K8EkSTVS1Wz_YoGsI1-mQ",
        "version" : {
          "created" : "7060299"
        },
        "provided_name" : "first_index"
      }
    }
  },
  "second_index" : {
    "aliases" : { },
    "mappings" : { },
    "settings" : {
      "index" : {
        "creation_date" : "1586692828618",
        "number_of_shards" : "1",
        "number_of_replicas" : "1",
        "uuid" : "Elvn5k7HRV2pM-xjrWFf5w",
        "version" : {
          "created" : "7060299"
        },
        "provided_name" : "second_index"
      }
    }
  }
}
```

### Cleanup

Run the following commands to cleanup the test resources that have been created throughout this tutorial,

```console
# delete all created resources from the source cluster
kubectl delete -f ./examples/user-guide/ --context=<src cluster context>

# delete all created resources from the destination cluster
kubectl delete -f ./examples/user-guide/ --context=<dst cluster context>

# delete all the DataSync CR from the source cluster
kubectl delete datasync --all --all-namespaces --context=<src cluster context>

# delete all the DataSync CR from the destination cluster
kubectl delete datasync --all --all-namespaces --context=<dst cluster context>
```

If you want to uninstall the plugin or Kubemove, follow the uninstall section of their setup guide.
