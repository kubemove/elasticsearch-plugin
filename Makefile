SHELL:=/bin/bash -o pipefail

REGISTRY?=kubemove
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

# Run gofmt and goimports in all packages except vendor
# Example: make format REGISTRY=<your docker registry>
.PHONY: format
format: $(BUILD_DIRS)
	@echo "Formatting repo...."
	@docker run                                                     			\
			-i                                                      			\
			--rm                                                    			\
			-u $$(id -u):$$(id -g)                                  			\
			-v $$(pwd):/go/src/$(MODULE)                            			\
			-v $$(pwd)/.go/cache:/.cache                            			\
			-w /go/src                                              			\
			--env HTTP_PROXY=$(HTTP_PROXY)                          			\
			--env HTTPS_PROXY=$(HTTPS_PROXY)                        			\
			--env GO111MODULE=on                                    			\
			--env GOFLAGS="-mod=vendor"                             			\
			$(BUILD_IMAGE)                                          			\
			/bin/sh -c "gofmt -s -w $(PACKAGES)	&& goimports -w $(PACKAGES)"
	@echo "Done"

# Run linter
# Example: make lint REGISTRY=<your docker registry>
ADDITIONAL_LINTERS   := goconst,gofmt,goimports,unparam,misspell
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
build: $(BUILD_DIRS) format
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
	@echo "Done"

# Push docker images into docker registry
# Example: make deploy-images REGISTRY=<your docker registry>
.PHONY: deploy-images
deploy-images:
	@echo ""
	@echo "Pushing $(PLUGIN_IMAGE) image...."
	@docker push $(PLUGIN_IMAGE)
	@echo "Done"

# Testing related Envs
KUBECONFIG?=
SRC_CONTEXT?=kind-src-cluster
DST_CONTEXT?=kind-dst-cluster

SRC_ES_NODE_PORT?=
DST_ES_NODE_PORT?=

MINIO_ACCESS_KEY=not@accesskey
MINIO_SECRET_KEY=not@secretkey

SRC_CLUSTER_IP:=
DST_CLUSTER_IP:=

.PHONY: setup-envs
setup-envs:
	$(eval SRC_CONTROL_PANE:=$(shell /bin/bash -c "kubectl get pod -n kube-system  -o name --context=$(SRC_CONTEXT)| grep kube-apiserver"))
	$(eval SRC_CLUSTER_IP=$(shell (kubectl get -n kube-system $(SRC_CONTROL_PANE) -o yaml --context=$(SRC_CONTEXT)| grep advertise-address= | cut -c27-)))
	$(eval DST_CONTROL_PANE:=$(shell /bin/bash -c "kubectl get pod -n kube-system  -o name --context=$(DST_CONTEXT)| grep kube-apiserver"))
	$(eval DST_CLUSTER_IP=$(shell (kubectl get -n kube-system $(DST_CONTROL_PANE) -o yaml --context=$(DST_CONTEXT)| grep advertise-address= | cut -c27-)))

# Install Elasticsearch Plugin in source and destination cluster
# Example: make install-plugin
.PHONY: install-plugin
install-plugin:
	@echo ""
	@echo "Installing Elasticsearch Plugin into the source cluster...."
	@cat deploy/plugin.yaml | envsubst | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo "Installing Elasticsearch Plugin into the destination cluster...."
	@cat deploy/plugin.yaml | envsubst | kubectl apply -f - --context=$(DST_CONTEXT)
	@echo "Done"

# Uninstall Elasticsearch Plugin from the source and destination cluster
# Example: make uninstall-plugin
.PHONY: uninstall-plugin
uninstall-plugin:
	@echo " "
	@kubectl delete -f deploy/plugin.yaml --context=$(SRC_CONTEXT) --wait=true || true
	@kubectl delete -f deploy/plugin.yaml --context=$(DST_CONTEXT) --wait=true || true
	@echo "Done"

# Deploy ECK operator in the source and destination cluster
# Example: make deploy-eck-operator
.PHONY: deploy-eck-operator
deploy-eck-operator:
	@echo ""
	@echo "Deploying ECK operator in the source cluster"
	@kubectl apply -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=$(SRC_CONTEXT)
	@echo ""
	@echo "Deploying ECK operator in the destination cluster"
	@kubectl apply -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=$(DST_CONTEXT)
	@echo "Done"

# Remove ECK operator from the source and destination cluster
# Example: make remove-eck-oerator
.PHONY: remove-eck-operator
remove-eck-operator:
	@echo ""
	@echo "Removing ECK operator in the source cluster"
	@kubectl delete -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=$(SRC_CONTEXT) --wait=true || true
	@echo ""
	@echo "Removing ECK operator in the destination cluster"
	@kubectl delete -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=$(DST_CONTEXT) --wait=true || true
	@echo "Done"

# Deploy a sample Elasticsearch in the source and destination cluster
# Example: make deploy-sample-es
.PHONY: deploy-sample-es
deploy-sample-es:
	@echo " "
	@echo "Deploying sample Easticsearch into the source cluster...."
	@kubectl apply -f examples/elasticsearch/sample_es.yaml --context=$(SRC_CONTEXT)
	@echo "Deploying sample Easticsearch into the destination cluster...."
	@kubectl apply -f examples/elasticsearch/sample_es.yaml --context=$(DST_CONTEXT)
	@/bin/bash -c "sleep 10"
	@kubectl wait --for=condition=READY pods sample-es-es-default-0 sample-es-es-default-1 --timeout=10m --context=$(SRC_CONTEXT)
	@kubectl wait --for=condition=READY pods sample-es-es-default-0 sample-es-es-default-1 --timeout=10m --context=$(DST_CONTEXT)
	@echo "Done"

# Remove the sample Elasticsearch from the source and destination cluster
# Example: make remove-sample-es
.PHONY: remove-sample-es
remove-sample-es:
	@echo " "
	@echo "Removing sample Easticsearch from the source cluster...."
	@kubectl delete -f examples/elasticsearch/sample_es.yaml --context=$(SRC_CONTEXT) --wait=true || true
	@echo "Removing sample Easticsearch from the destination cluster...."
	@kubectl delete -f examples/elasticsearch/sample_es.yaml --context=$(DST_CONTEXT) --wait=true || true
	@echo "Done"

# Deploy a Minio server in the destination cluster
# Example: make deploy-minio-server
.PHONY: deploy-minio-server
deploy-minio-server:
	@echo ""
	@echo "Creating Minio secret into the source cluster...."
	@cat examples/repository/minio_secret.yaml | \
		MINIO_ACCESS_KEY=$(MINIO_ACCESS_KEY)     \
		MINIO_SECRET_KEY=$(MINIO_SECRET_KEY)     \
		envsubst | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo "Creating Minio secret into the destination cluster...."
	@cat examples/repository/minio_secret.yaml | \
		MINIO_ACCESS_KEY=$(MINIO_ACCESS_KEY)     \
		MINIO_SECRET_KEY=$(MINIO_SECRET_KEY)     \
    	envsubst | kubectl apply -f - --context=$(DST_CONTEXT)
	@echo "Deploying Minio server into the destination cluster...."
	@cat examples/repository/minio_server.yaml | envsubst | kubectl apply -f - --context=$(DST_CONTEXT)
	@echo "Done"

# Remove the Minio server and its associated resources
# Example: make remove-minio-server
.PHONY: remove-minio-server
remove-minio-server:
	@echo ""
	@echo "Removing Minio server from the destination cluster...."
	@kubectl delete -f examples/repository/minio_server.yaml --context=$(DST_CONTEXT) --wait=true || true
	@echo " Deleting Minio secret from the source cluster...."
	@kubectl delete -f examples/repository/minio_secret.yaml --context=$(SRC_CONTEXT) || true
	@echo " Deleting Minio secret from the destination cluster...."
	@kubectl delete -f examples/repository/minio_secret.yaml --context=$(DST_CONTEXT) || true
	@echo "Done"

MINO_NODEPORT=
MINIO_SERVER_ADDRESS=
BUCKET_NAME=demo
.PHONY: create-minio-bucket
create-minio-bucket: setup-envs
	@echo " "
	$(eval MINIO_NODEPORT:=$(shell (kubectl get service minio -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval MINIO_SERVER_ADDRESS:=$(DST_CLUSTER_IP):$(MINIO_NODEPORT))
	@docker run                                                     \
			-i                                                      \
			--rm                                                    \
			--net=host                                              \
			-u $$(id -u):$$(id -g)                                  \
			-v $$(pwd):/go/src/$(MODULE)                            \
			-v $$(pwd)/.go/cache:/.cache                            \
			-v $(HOME)/.kube:/.kube                                 \
			-v /tmp:/.mc											\
			-w /go/src/$(MODULE)                                    \
			--env HTTP_PROXY=$(HTTP_PROXY)                          \
			--env HTTPS_PROXY=$(HTTPS_PROXY)                        \
			--env GINKGO_ARGS=$(GINKGO_ARGS)                        \
			$(BUILD_IMAGE)                                          \
			/bin/bash -c "                                          \
				mc config host add es-repo http://$(MINIO_SERVER_ADDRESS) $(MINIO_ACCESS_KEY) $(MINIO_SECRET_KEY) && \
				mc mb es-repo/$(BUCKET_NAME) 																		 \
			"

# Create MoveEngine CR to sync data between two sample Elasticsearch
# Example: make setup-sample-es-sync
.PHONY: setup-sample-es-sync
setup-sample-es-sync: setup-envs
	@echo " "
	$(eval MINIO_NODEPORT:=$(shell (kubectl get service minio -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval MINIO_SERVER_ADDRESS:=$(DST_CLUSTER_IP):$(MINIO_NODEPORT))

	@echo "Creating MoveEngine CR into the source cluster...."
	@cat examples/moveengine/sample_es_move.yaml |                   \
		MINIO_SERVER_ADDRESS=$(MINIO_SERVER_ADDRESS)                 \
		envsubst | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo "Done"

# Create MoveEngine CR to sync data between two sample Elasticsearch
# Example: make remove-sample-es-sync
.PHONY: remove-sample-es-sync
remove-sample-es-sync:
	@echo ""
	@echo "Removing MoveEngine CR from the source cluster...."
	@kubectl delete -f examples/moveengine/sample_es_move.yaml --context=$(SRC_CONTEXT) || true
	@echo "Removing MoveEngine CR from the destination cluster...."
	@kubectl delete -f examples/moveengine/sample_es_move.yaml --context=$(DST_CONTEXT) || true
	@echo "Done"

# Trigger INIT API
#Example: make trigger-init
.PHONY: trigger-init
trigger-init: setup-envs
	$(eval SRC_PLUGIN_NODEPORT:=$(shell (kubectl get service elasticsearch-plugin -o yaml --context=$(SRC_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval SRC_PLUGIN_ADDRESS:=$(SRC_CLUSTER_IP):$(SRC_PLUGIN_NODEPORT))
	$(eval DST_PLUGIN_NODEPORT:=$(shell (kubectl get service elasticsearch-plugin -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_PLUGIN_ADDRESS:=$(DST_CLUSTER_IP):$(DST_PLUGIN_NODEPORT))

	@build/bin/elasticsearch-plugin run trigger-init \
		--kubeconfig=$(KUBECONFIG)                   \
		--src-context=$(SRC_CONTEXT)                 \
		--dst-context=$(DST_CONTEXT)                 \
		--src-plugin=$(SRC_PLUGIN_ADDRESS)           \
		--dst-plugin=$(DST_PLUGIN_ADDRESS)

# Trigger SYNC API
# Example: make trigger-sync
.PHONY: trigger-sync
trigger-sync: setup-envs
	$(eval SRC_PLUGIN_NODEPORT:=$(shell (kubectl get service elasticsearch-plugin -o yaml --context=$(SRC_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval SRC_PLUGIN_ADDRESS:=$(SRC_CLUSTER_IP):$(SRC_PLUGIN_NODEPORT))
	$(eval DST_PLUGIN_NODEPORT:=$(shell (kubectl get service elasticsearch-plugin -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_PLUGIN_ADDRESS:=$(DST_CLUSTER_IP):$(DST_PLUGIN_NODEPORT))

	@build/bin/elasticsearch-plugin run trigger-sync \
		--kubeconfig=$(KUBECONFIG)                   \
		--src-context=$(SRC_CONTEXT)                 \
		--dst-context=$(DST_CONTEXT)                 \
		--src-plugin=$(SRC_PLUGIN_ADDRESS)           \
		--dst-plugin=$(DST_PLUGIN_ADDRESS)

# Insert a index in the source cluster
# Example: make insert-index INDEX_NAME=my-index
ES_NAME?=sample-es
ES_NAMESPACE=default
INDEX_NAME?=test-index
.PHONY: insert-index
insert-index: setup-envs
	$(eval SRC_ES_NODEPORT:=$(shell (kubectl get service sample-es-es-http -o yaml --context=$(SRC_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_ES_NODEPORT:=$(shell (kubectl get service sample-es-es-http -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))

	@build/bin/elasticsearch-plugin run insert-index \
		--kubeconfig=$(KUBECONFIG)                   \
		--src-context=$(SRC_CONTEXT)                 \
		--dst-context=$(DST_CONTEXT)                 \
		--src-cluster-ip=$(SRC_CLUSTER_IP)           \
		--dst-cluster-ip=$(DST_SRC_CLUSTER_IP)       \
		--src-es-nodeport=$(SRC_ES_NODEPORT)         \
		--dst-es-nodeport=$(DST_ES_NODEPORT)         \
		--es-name=$(ES_NAME)                         \
		--es-namespace=$(ES_NAMESPACE)               \
		--index-name=$(INDEX_NAME)

# Show all indexes from the targeted ES
# Example: make show-indexes FROM=active
FROM?=active
.PHONY: show-indexes
show-indexes: setup-envs
	$(eval SRC_ES_NODEPORT:=$(shell (kubectl get service sample-es-es-http -o yaml --context=$(SRC_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_ES_NODEPORT:=$(shell (kubectl get service sample-es-es-http -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval DST_PLUGIN_ADDRESS:=$(DST_CLUSTER_IP):$(DST_PLUGIN_NODEPORT))

	@build/bin/elasticsearch-plugin run show-indexes \
		--kubeconfig=$(KUBECONFIG)                   \
		--src-context=$(SRC_CONTEXT)                 \
		--dst-context=$(DST_CONTEXT)                 \
		--src-cluster-ip=$(SRC_CLUSTER_IP)           \
		--dst-cluster-ip=$(DST_CLUSTER_IP)           \
		--src-es-nodeport=$(SRC_ES_NODEPORT)         \
		--dst-es-nodeport=$(DST_ES_NODEPORT)         \
		--es-name=$(ES_NAME)                         \
        --es-namespace=$(ES_NAMESPACE)               \
		--index-from=$(FROM)

# Run E2E tests.
# Before running ths command make sure you have run "make prepare", "make install-plugin", "make deploy-es" and "make setup-sync"
# Example: make e2e-test
GINKGO_ARGS ?= "--flakeAttempts=1"
e2e-test:
	@docker run                                                     \
			-i                                                      \
			--rm                                                    \
			--net=host                                              \
			-u $$(id -u):$$(id -g)                                  \
			-v $$(pwd):/go/src/$(MODULE)                            \
			-v $$(pwd)/.go/cache:/.cache                            \
			-v $(HOME)/.kube:/.kube                                 \
			-v /tmp:/.mc											\
			-w /go/src/$(MODULE)                                    \
			--env HTTP_PROXY=$(HTTP_PROXY)                          \
			--env HTTPS_PROXY=$(HTTPS_PROXY)                        \
			--env KUBECONFIG=$(KUBECONFIG)                          \
			--env GO111MODULE=on                                    \
			--env GOFLAGS="-mod=vendor"                             \
			--env SRC_CONTEXT=$(SRC_CONTEXT)                        \
			--env DST_CONTEXT=$(DST_CONTEXT)                        \
			--env GINKGO_ARGS=$(GINKGO_ARGS)                        \
			$(BUILD_IMAGE)                                          \
			/bin/bash -c "                                          \
				KUBECONFIG=$${KUBECONFIG#$(HOME)}                   \
				./hack/e2e_test.sh		                            \
				"
