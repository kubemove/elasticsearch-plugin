# Elasticsearch Plugin for Kubemove

This is a plugin for [Kubemove](https://github.com/kubemove/kubemove) to sync data between two Elasticsearch clusters deployed with [ECK Operator](https://github.com/elastic/cloud-on-k8s).

## Run

Here we are going to give a brief instruction on how you can install this plugin and test it's functionality.

- **Set Max Map Count:** Elasticsearch requires large memory number of memory map areas. Give the VM to access larger memory areas.
    ```console
    sudo sysctl -w vm.max_map_count=262144
    ```

- **Configure Docker Registry:** Set REGISTRY env to use your own docker registry.
    ```console
    export REGISTRY=<your docker registry>
    ```

- **Create Clusters:** Now, create two cluster using kind. One is `src-cluster` another is `dst-cluster`.
    ```console
    kind create cluster --name=src-cluster
    kind create cluster --name=dst-cluster
    ```

- **Prepare Clusters:** Prepare the clusters before using the plugin.
    ```console
    make prepare REGISTRY=emruzhossain SRC_CONTEXT=kind-src-cluster DST_CONTEXT=kind-dst-cluster
    ```
    This will do the following things:
    - Install ECK operator in the both clusters.
    - Register Kubemove CRDs in the both clusters.
    - Create necessary RBAC stuffs in the both clusters.
    - Install DataSync and MoveEngine controller in the both clusters.
    - Install a Minio server in the destination cluster and create a    bucket named `demo` there.

- **Install Elasticsearch Plugin:** Now, install the Elasticsearch plugin in the both clusters.
    ```console
    make install-plugin REGISTRY=emruzhossain
    ```

- **Deploy Elasticsearch:** Deploy Elasticsearch in the both clusters. Don't forget to wait until the Elaticsearch databases are ready.
    ```console
    make deploy-es
    ```

- **Configure Sync:** Its time to setup a sync between the Elasticsearces. The command shown below will create MoveEngine CR in the both clusters. MoveEngine CR of source cluster will have `active` mode and MoveEngine CR of the destination cluster will have `standby` mode.
    ```console
    make setup-sync
    ```

- **Trigger INIT API:** Now, Trigger INIT API. This will install repository plugin into the both Elasticsearches. Then, it will register the minio repository in them.
    ```console
    make trigger-init
    ```

- **Insert Index:** Create a demo index in the source cluster.
  ```console
  make insert-index INDEX_NAME=demo-index
  ```

- **Verify Sample Data:** Verify that the sample data has been successfully inserted in the source ES.
    ```console
    make show-indexes FROM=active
    ```

- **Trigger SYNC API:** Now, trigger a sync between the ES clusters.
    ```console
    make trigger-sync
    ```

- **Verify Synced Data:** Now, check whether the demo-index present in the destination cluster.
    ```console
    make show-indexes FROM=standby
    ```

## Developer Guide

Here are few tricks to help with working with this project.

**Dependencies:**

You will need `emruzhossain/kubemove-dev:1.13.8` docker image. At first, you have to re-tag the image and upload to your own registry.

```console
docker tag emruzhossain/kubemove-dev:1.13.8 <your registry>/kubemove-dev:1.13.8
```

Then, set `REGISTRY` env to use your docker registry for further commands.

```console
export REGISTRY=<your docker registry>
```

**Helper Instructions:**

- Run `gofmt` and `goimports`:
    ```console
    make format
    ```

- Run linter:
    ```console
    make lint
    ```

- Revendor project dependencies:
    ```console
    make revendor
    ```

- Build project:
    ```console
    make build
    ```

- Make plugin docker image:
    ```console
    make plugin-image
    ```

- Update CRDs:
    ```console
    make update-crds ORG=<your username> BRANCH=<your branch>
    ```

- Run e2e tests:
    ```console
    make e2e-test
    ```

- Remove all plugin resources from your cluster
    ```console
    make reset
    ```
