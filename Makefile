SHELL:=/bin/bash -o pipefail

REGISTRY?=kubemovedev
REPO_ROOT:=$(shell pwd)
GO_VERSION?=1.13.8
BUILD_IMAGE?= $(REGISTRY)/kubemove-dev:$(GO_VERSION)

MODULE=github.com/kubemove/elasticsearch-plugin
PACKAGES = $(shell go list ./... | grep -v 'vendor')


# Use IMAGE_TAG to <branch-name>-<commit-hash> to make sure that
# one dev don't overwrite another's image in the docker registry
git_branch       := $(shell git rev-parse --abbrev-ref HEAD)
git_tag          := $(shell git describe --exact-match --abbrev=0 2>/dev/null || echo "")
commit_hash      := $(shell git rev-parse --verify --short HEAD)

ifdef git_tag
	IMAGE_TAG = $(git_tag)
else
	IMAGE_TAG = $(git_branch)-$(commit_hash)
endif

# Directories required to build the project
BUILD_DIRS:= build/bin \
			.go/cache
# Create the directories provided in BUILD_DIRS
$(BUILD_DIRS):
	@mkdir -p $@

# Run go fmt in all packages except vendor
# Example: make fmt REGISTRY=<your docker registry>
.PHONY: fmt
fmt: $(BUILD_DIRS)
	@echo "Running go fmt...."
	@docker run                                                     \
			-i                                                      \
			--rm                                                    \
			-u $$(id -u):$$(id -g)                                  \
			-v $$(pwd):/go/src/$(MODULE)                            \
			-v $$(pwd)/.go/cache:/.cache                            \
			-w /go/src                                              \
			--env HTTP_PROXY=$(HTTP_PROXY)                          \
			--env HTTPS_PROXY=$(HTTPS_PROXY)                        \
			--env GO111MODULE=on                                    \
			--env GOFLAGS="-mod=vendor"                             \
			$(BUILD_IMAGE)                                          \
			gofmt -s -w $(PACKAGES)
	@echo "Done"

# Run linter
# Example: make lint REGISTRY=<your docker registry>
ADDITIONAL_LINTERS   := goconst,gofmt,goimports,unparam
.PHONY: lint
lint: $(BUILD_DIRS)
	@echo "Running go lint....."
	@docker run                                                     \
			-i                                                      \
			--rm                                                    \
			-u $$(id -u):$$(id -g)                                  \
			-v $$(pwd):/go/src/$(MODULE)                            \
			-v $$(pwd)/.go/cache:/.cache                            \
			-w /go/src/$(MODULE)                                    \
			--env HTTP_PROXY=$(HTTP_PROXY)                          \
			--env HTTPS_PROXY=$(HTTPS_PROXY)                        \
			--env GO111MODULE=on                                    \
			--env GOFLAGS="-mod=vendor"                             \
			$(BUILD_IMAGE)                                          \
			golangci-lint run                                       \
			--enable $(ADDITIONAL_LINTERS)                          \
			--timeout=10m                                           \
			--skip-dirs-use-default                                 \
			--skip-dirs=vendor
	@echo "Done"

# Update the dependencies
# Example: make revendor
.PHONY: revendor
revendor:
	@echo "Revendoring project....."
	@docker run                                                     \
			-i                                                      \
			--rm                                                    \
			-u $$(id -u):$$(id -g)                                  \
			-v $$(pwd):/go/src/$(MODULE)                            \
			-v $$(pwd)/.go/cache:/.cache                            \
			-w /go/src/$(MODULE)                                    \
			--env HTTP_PROXY=$(HTTP_PROXY)                          \
			--env HTTPS_PROXY=$(HTTPS_PROXY)                        \
			--env GO111MODULE=on                                    \
			--env GOFLAGS="-mod=vendor"                             \
			$(BUILD_IMAGE)                                          \
			/bin/sh -c "go mod vendor && go mod tidy"
	@echo "Done"

# Clean old binaries and build caches
# Example: make clean
.PHONY: clean
clean:
	@echo "Cleaning old binaries and build caches...."
	@rm -rf $(BUILD_DIRS)
	@echo "Done"

# Build Binary
# Example: make build
.PHONY: build
build: $(BUILD_DIRS) fmt
	@echo "Building Elasticsearch Plugin...."
	@docker run                                                     \
			-i                                                      \
			--rm                                                    \
			-u $$(id -u):$$(id -g)                                  \
			-v $$(pwd):/go/src/$(MODULE)                            \
			-v $$(pwd)/.go/cache:/.cache                            \
			-w /go/src/$(MODULE)                                    \
			--env HTTP_PROXY=$(HTTP_PROXY)                          \
			--env HTTPS_PROXY=$(HTTPS_PROXY)                        \
			--env GO111MODULE=on                                    \
			--env GOFLAGS="-mod=vendor"                             \
			$(BUILD_IMAGE)                                          \
			/bin/bash -c "go build -o build/bin/elasticsearch-plugin cmd/main.go"
	@echo "Done"

# Build plugin image and push in the docker registry
# Example: make plugin-image REGISTRY=<your docker registry>
export PLUGIN_IMAGE:=$(REGISTRY)/elasticsearch-plugin:$(IMAGE_TAG)
.PHONY: plugin-image
plugin-image: build
	@echo ""
	@echo "Building Elasticsearch plugin docker image"
	@docker build -t $(PLUGIN_IMAGE) -f build/Dockerfile ./build
	@echo "Successfully built $(PLUGIN_IMAGE)"
	@echo ""
	@echo "Pushing $(PLUGIN_IMAGE) image...."
	@docker push $(PLUGIN_IMAGE)

.PHONY: update-crds
update-crds:

# Testing related Environments
KUBECONFIG_PATH?=
SRC_CONTEXT?=kind-src-cluster
DST_CONTEXT?=kind-dst-cluster


SRC_ES_NODE_PORT?=
DST_ES_NODE_PORT?=

SRC_CONTROL_PANE:=$(shell /bin/bash -c "kubectl get pod -n kube-system  -o name --context=$(SRC_CONTEXT)| grep kube-apiserver")
SRC_CLUSTER_IP=$(shell (kubectl get -n kube-system $(SRC_CONTROL_PANE) -o yaml --context=$(SRC_CONTEXT)| grep advertise-address= | cut -c27-))
DST_CONTROL_PANE:=$(shell /bin/bash -c "kubectl get pod -n kube-system  -o name --context=$(DST_CONTEXT)| grep kube-apiserver")
DST_CLUSTER_IP=$(shell (kubectl get -n kube-system $(DST_CONTROL_PANE) -o yaml --context=$(DST_CONTEXT)| grep advertise-address= | cut -c27-))

.PHONY: cluster_ip
cluster_ip:
	@echo $(SRC_CLUSTER_IP)
	@echo $(DST_CLUSTER_IP)

# Install all dependencies for testing
# Example: make prepare REGISTRY=<your docker registry> KUBECONFIG_PATH=<kubeconfig path> SRC_CONTEXT=<source context> DST_CONTEXT=<destination context>
.PHONY: prepare
prepare:
	@echo "Installing ECK operator in the source cluster"
	@kubectl apply -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=$(SRC_CONTEXT)
	@echo "Installing ECK operator in the destination cluster"
	@kubectl apply -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=$(DST_CONTEXT)
	@echo ""
	@echo "Registering Kubemove CRDs in the source cluster"
	@kubectl apply -f deploy/dependencies/crds/ --context=$(SRC_CONTEXT)
	@echo "Registering Kubemove CRDs in the destination cluster"
	@kubectl apply -f deploy/dependencies/crds/ --context=$(DST_CONTEXT)
	@echo ""
	@echo "Creating RBAC resources in the source cluster"
	@kubectl apply -f deploy/dependencies/rbac.yaml --context=$(SRC_CONTEXT)
	@echo "Creating RBAC resources in the destination cluster"
	@kubectl apply -f deploy/dependencies/rbac.yaml --context=$(DST_CONTEXT)
	@echo ""
	@echo "Installing DataSync controller in the source cluster"
	@kubectl apply -f deploy/dependencies/datasync_controller.yaml --context=$(SRC_CONTEXT)
	@echo "Installing DataSync controller in the destination cluster"
	@kubectl apply -f deploy/dependencies/datasync_controller.yaml --context=$(DST_CONTEXT)
	@echo ""
	@echo "Installing MoveEngine controller in the source cluster"
	@kubectl apply -f deploy/dependencies/moveengine_controller.yaml --context=$(SRC_CONTEXT)
	@echo "Installing MoveEngine controller in the destination cluster"
	@kubectl apply -f deploy/dependencies/moveengine_controller.yaml --context=$(DST_CONTEXT)
	@echo ""
	@echo "Creating Minio Secret in the source cluster"
	@kubectl apply -f deploy/dependencies/minio_secret.yaml --context=$(SRC_CONTEXT)
	@echo "Creating Minio Secret in the destination cluster"
	@kubectl apply -f deploy/dependencies/minio_secret.yaml --context=$(DST_CONTEXT)
	@echo ""
	@echo "Deploying Minio Server in the destination cluster"
	@kubectl apply -f deploy/dependencies/minio_server.yaml --context=$(DST_CONTEXT)

# Install Elasticsearch Plugin in source and destination cluster
# Example: make install-plugin
.PHONY: install-plugin
install-plugin:
	@echo "Installing Elasticsearch Plugin into the source cluster...."
	@cat deploy/plugin.yaml | envsubst | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo " "
	@echo "Installing Elasticsearch Plugin into the destination cluster...."
	@cat deploy/plugin.yaml | envsubst | kubectl apply -f - --context=$(DST_CONTEXT)

# Deploy Elasticsearch Plugin in source and destination cluster
# Example: make deploy-es
.PHONY: deploy-es
deploy-es:
	@echo "Deploying sample Easticsearch into the source cluster...."
	@cat deploy/elasticsearch.yaml | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo " "
	@echo "Deploying sample Easticsearch into the destination cluster...."
	@cat deploy/elasticsearch.yaml | kubectl apply -f - --context=$(DST_CONTEXT)

# Create MoveEngine CR to sync data between two Elasticsearch
# Example: make stup-sync
.PHONY: setup-sync
setup-sync:
	$(eval MINIO_NODEPORT:=$(shell (kubectl get service minio -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval MINIO_SERVER_ADDRESS:=$(DST_CLUSTER_IP):$(MINIO_NODEPORT))

	@echo "Creating MoveEngine CR into the source cluster...."
	@cat deploy/moveengine.yaml | \
		MODE=active \
		MINIO_SERVER_ADDRESS=$(MINIO_SERVER_ADDRESS)\
		envsubst | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo " "
	@echo "Creating MoveEngine CR into the destination cluster...."
	@cat deploy/moveengine.yaml | \
    		MODE=standby \
    		MINIO_SERVER_ADDRESS=$(MINIO_SERVER_ADDRESS)\
    		envsubst | kubectl apply -f - --context=$(DST_CONTEXT)

# Trigger INIT API
#Example: make trigger-init
.PHONY: trigger-init
trigger-init:
	$(eval SRC_PLUGIN_NODEPORT:=$(shell (kubectl get service eck-plugin -o yaml --context=$(SRC_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval SRC_PLUGIN_ADDRESS:=$(SRC_CLUSTER_IP):$(SRC_PLUGIN_NODEPORT))
	$(eval DST_PLUGIN_NODEPORT:=$(shell (kubectl get service eck-plugin -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_PLUGIN_ADDRESS:=$(DST_CLUSTER_IP):$(DST_PLUGIN_NODEPORT))

	@go run test/main.go trigger-init \
		--kubeconfigpath=$(KUBECONFIG_PATH)  \
		--src-context=$(SRC_CONTEXT) \
		--dst-context=$(DST_CONTEXT) \
		--src-plugin=$(SRC_PLUGIN_ADDRESS) \
		--dst-plugin=$(DST_PLUGIN_ADDRESS)

# Trigger SYNC API
# Example: make trigger-sync
.PHONY: trigger-sync
trigger-sync:
	$(eval SRC_PLUGIN_NODEPORT:=$(shell (kubectl get service eck-plugin -o yaml --context=$(SRC_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval SRC_PLUGIN_ADDRESS:=$(SRC_CLUSTER_IP):$(SRC_PLUGIN_NODEPORT))
	$(eval DST_PLUGIN_NODEPORT:=$(shell (kubectl get service eck-plugin -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_PLUGIN_ADDRESS:=$(DST_CLUSTER_IP):$(DST_PLUGIN_NODEPORT))

	@go run test/main.go trigger-sync \
		--kubeconfigpath=$(KUBECONFIG_PATH)  \
		--src-context=$(SRC_CONTEXT) \
		--dst-context=$(DST_CONTEXT) \
		--src-plugin=$(SRC_PLUGIN_ADDRESS) \
		--dst-plugin=$(DST_PLUGIN_ADDRESS)

# Insert a index in the source cluster
# Example: make insert-index INDEX_NAME=my-index
INDEX_NAME?=test-index
.PHONY: insert-index
insert-index:
	$(eval SRC_ES_NODEPORT:=$(shell (kubectl get service sample-es-es-http -o yaml --context=$(SRC_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_ES_NODEPORT:=$(shell (kubectl get service sample-es-es-http -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))

	@go run test/main.go insert-index \
		--kubeconfigpath=$(KUBECONFIG_PATH)  \
		--src-context=$(SRC_CONTEXT) \
		--dst-context=$(DST_CONTEXT) \
		--src-cluster-ip=$(SRC_CLUSTER_IP) \
		--dst-cluster-ip=$(DST_SRC_CLUSTER_IP) \
		--src-es-nodeport=$(SRC_ES_NODEPORT) \
		--dst-es-nodeport=$(DST_ES_NODEPORT) \
		--index-name=$(INDEX_NAME)

# Show all indexes from the targeted ES
# Example: make show-indexes FROM=active
FROM?=active
.PHONY: show-indexes
show-indexes:
	$(eval SRC_ES_NODEPORT:=$(shell (kubectl get service sample-es-es-http -o yaml --context=$(SRC_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_ES_NODEPORT:=$(shell (kubectl get service sample-es-es-http -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_PLUGIN_ADDRESS:=$(DST_CLUSTER_IP):$(DST_PLUGIN_NODEPORT))

	@go run test/main.go show-indexes \
		--kubeconfigpath=$(KUBECONFIG_PATH)  \
		--src-context=$(SRC_CONTEXT) \
		--dst-context=$(DST_CONTEXT) \
		--src-cluster-ip=$(SRC_CLUSTER_IP) \
		--dst-cluster-ip=$(DST_CLUSTER_IP) \
		--src-es-nodeport=$(SRC_ES_NODEPORT) \
		--dst-es-nodeport=$(DST_ES_NODEPORT) \
		--index-from=$(FROM)
