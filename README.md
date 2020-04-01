# Elasticsearch Plugin

This is a plugin for [Kubemove](https://github.com/kubemove/kubemove) to sync data between two Elasticsearch clusters deployed with [ECK Operator](https://github.com/elastic/cloud-on-k8s).

## Pre-requisite

You must install [Kubemove](https://github.com/kubemove/kubemove) installed before installing this plugin.  In order to install Kubemove framework, please for the guide [here](https://github.com/kubemove/kubemove#deploy-kubemove).

## Install Plugin

At first, export your source cluster's and destination cluster's context.

```console
export SRC_CONTEXT=<source cluster context>
export DST_CONTEXT=<destination cluster context>
```

Then, run the following command to install the Elasticsearch Plugin in both of your source and destination cluster.

```console
make install-plugin
```

## Uninstall Plugin

In order to uninstall Elasticsearch plugin from your cluster, run the following command.

```console
make uninstall-plugin
```

## Developer Guide

Here are few tricks to help with working with this project.

### Setup Environment

Setup your development environment by the following steps.

- Use your own docker account for the docker images:
    ```console
    export REGISTRY=<your docker username>
    ```

- Checkout into `kubemove/kubemove` repository and build the developer image with all dependencies:
    ```console
    make dev-image
    ```

### Build Project

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
- Push plugin docker image into your docker registry:
    ```console
    make deploy-images
    ```


### Test

Follow the following steps to run the E2E tests.

- At first, install the Kubemove framework.
- Install Elasticsearch plugin.
- Run e2e tests:
    ```console
    make e2e-test
    ```

There are also some other commands that automate many of the tasks that you  will need to do to test this plugin functionalities.
Check them on the `Makefile`.
