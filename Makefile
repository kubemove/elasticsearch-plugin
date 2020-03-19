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
	@echo ""
	@echo "Pushing $(PLUGIN_IMAGE) image...."
	@docker push $(PLUGIN_IMAGE)


# Update CRDs to latest version kubemove repo
# Example: make update-crds ORG=<your username> BRANCH=<your branch>
ORG?=kubemove
BRANCH?=master
CRD_SRC:=https://raw.githubusercontent.com/$(ORG)/kubemove/$(BRANCH)/deploy/crds/
.PHONY: update-crds
update-crds:
	@echo "Updating CRD definition....."
	@curl -fsSL $(CRD_SRC)/kubemove.io_datasyncs_crd.yaml    > deploy/dependencies/crds/kubemove.io_datasyncs_crd.yaml
	@curl -fsSL $(CRD_SRC)/kubemove.io_moveengines_crd.yaml  > deploy/dependencies/crds/kubemove.io_moveengines_crd.yaml
	@curl -fsSL $(CRD_SRC)/kubemove.io_movepairs_crd.yaml    > deploy/dependencies/crds/kubemove.io_movepairs_crd.yaml
	@curl -fsSL $(CRD_SRC)/kubemove.io_movereverses_crd.yaml > deploy/dependencies/crds/kubemove.io_movereverses_crd.yaml
	@curl -fsSL $(CRD_SRC)/kubemove.io_moveswitches_crd.yaml > deploy/dependencies/crds/kubemove.io_moveswitches_crd.yaml
	@echo "Done"

# Testing related Environments
KUBECONFIG?=
SRC_CONTEXT?=kind-src-cluster
DST_CONTEXT?=kind-dst-cluster


SRC_ES_NODE_PORT?=
DST_ES_NODE_PORT?=

SRC_CONTROL_PANE:=$(shell /bin/bash -c "kubectl get pod -n kube-system  -o name --context=$(SRC_CONTEXT)| grep kube-apiserver")
SRC_CLUSTER_IP=$(shell (kubectl get -n kube-system $(SRC_CONTROL_PANE) -o yaml --context=$(SRC_CONTEXT)| grep advertise-address= | cut -c27-))
DST_CONTROL_PANE:=$(shell /bin/bash -c "kubectl get pod -n kube-system  -o name --context=$(DST_CONTEXT)| grep kube-apiserver")
DST_CLUSTER_IP=$(shell (kubectl get -n kube-system $(DST_CONTROL_PANE) -o yaml --context=$(DST_CONTEXT)| grep advertise-address= | cut -c27-))

export MINIO_ACCESS_KEY=not@id
export MINIO_SECRET_KEY=not@secret


MINO_NODEPORT=
MINIO_SERVER_ADDRESS=

# Install all dependencies for testing
# Example: make prepare REGISTRY=<your docker registry> KUBECONFIG=<kubeconfig path> SRC_CONTEXT=<source context> DST_CONTEXT=<destination context>
.PHONY: prepare
prepare: plugin-image
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
			--env REGISTRY=$(REGISTRY)                              \
			--env MINIO_ACCESS_KEY=$(MINIO_ACCESS_KEY) 				\
			--env MINIO_SECRET_KEY=$(MINIO_SECRET_KEY) 				\
			--env SRC_CONTEXT=$(SRC_CONTEXT)                        \
			--env DST_CONTEXT=$(DST_CONTEXT)                        \
			$(BUILD_IMAGE)                                          \
			/bin/bash -c "                                          \
				KUBECONFIG=$${KUBECONFIG#$(HOME)}                   \
				./hack/prepare.sh		                            \
				"

# Install Elasticsearch Plugin in source and destination cluster
# Example: make install-plugin
.PHONY: install-plugin
install-plugin:
	@echo " "
	@echo "Installing Elasticsearch Plugin into the source cluster...."
	@cat deploy/plugin.yaml | envsubst | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo "Installing Elasticsearch Plugin into the destination cluster...."
	@cat deploy/plugin.yaml | envsubst | kubectl apply -f - --context=$(DST_CONTEXT)

.PHONY: uninstall-plugin #TODO: remove moveengine and datasync controller removal part
uninstall-plugin:
	@echo " "
	@kubectl delete -f deploy/plugin.yaml --context=$(SRC_CONTEXT)
	@kubectl delete -f deploy/plugin.yaml --context=$(DST_CONTEXT)

# Install Elasticsearch Plugin in source and destination cluster
# Example: make install-plugin
.PHONY: reinstall-plugin
reinstall-plugin: uninstall-plugin install-plugin
	@kubectl delete pods -n kubemove --all --context=${SRC_CONTEXT}
	@kubectl delete pods -n kubemove --all --context=${DST_CONTEXT}
	@kubectl wait --for=condition=READY pods --all --timeout=5m --context=${SRC_CONTEXT} || true
	@kubectl wait --for=condition=READY pods --all --timeout=5m --context=${DST_CONTEXT} || true

# Deploy Elasticsearch Plugin in source and destination cluster
# Example: make deploy-es
.PHONY: deploy-es
deploy-es:
	@echo " "
	@echo "Deploying sample Easticsearch into the source cluster...."
	@cat deploy/elasticsearch.yaml | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo "Deploying sample Easticsearch into the destination cluster...."
	@cat deploy/elasticsearch.yaml | kubectl apply -f - --context=$(DST_CONTEXT)

.PHONY: remove-es
remove-es:
	@echo " "
	@echo "Removing sample Easticsearch from the source cluster...."
	@kubectl delete -f deploy/elasticsearch.yaml --context=$(SRC_CONTEXT)
	@echo "Removing sample Easticsearch from the destination cluster...."
	@kubectl delete -f deploy/elasticsearch.yaml --context=$(DST_CONTEXT)
	@kubectl wait --for=condition=READY pods --all --timeout=5m --context=${SRC_CONTEXT} || true
	@kubectl wait --for=condition=READY pods --all --timeout=5m --context=${DST_CONTEXT} || true

.PHONY: redeploy-es
redeploy-es: remove-es deploy-es

# Create MoveEngine CR to sync data between two Elasticsearch
# Example: make stup-sync
.PHONY: setup-sync
setup-sync:
	@echo " "
	$(eval MINIO_NODEPORT:=$(shell (kubectl get service minio -o yaml --context=$(DST_CONTEXT) | grep nodePort | cut -c15-)))
	$(eval MINIO_SERVER_ADDRESS:=$(DST_CLUSTER_IP):$(MINIO_NODEPORT))

	@echo "Creating MoveEngine CR into the source cluster...."
	@cat deploy/moveengine.yaml |                                    \
		MODE=active                                                  \
		MINIO_SERVER_ADDRESS=$(MINIO_SERVER_ADDRESS)                 \
		envsubst | kubectl apply -f - --context=$(SRC_CONTEXT)
	@echo "Creating MoveEngine CR into the destination cluster...."
	@cat deploy/moveengine.yaml |                                    \
		MODE=standby                                                 \
		MINIO_SERVER_ADDRESS=$(MINIO_SERVER_ADDRESS)                 \
		envsubst | kubectl apply -f - --context=$(DST_CONTEXT)

# Trigger INIT API
#Example: make trigger-init
.PHONY: trigger-init
trigger-init:
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
trigger-sync:
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
INDEX_NAME?=test-index
.PHONY: insert-index
insert-index:
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
		--index-name=$(INDEX_NAME)

# Show all indexes from the targeted ES
# Example: make show-indexes FROM=active
FROM?=active
.PHONY: show-indexes
show-indexes:
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
				./hack/e2e-test.sh		                            \
				"

# Remove all the resources installed for testing this plugin
# Example: make reset
reset:
	@docker run                                                     \
			-i                                                      \
			--rm                                                    \
			--net=host                                              \
			-u $$(id -u):$$(id -g)                                  \
			-v $$(pwd):/go/src/$(MODULE)                            \
			-v $(HOME)/.kube:/.kube                                 \
			-w /go/src/$(MODULE)                                    \
			--env HTTP_PROXY=$(HTTP_PROXY)                          \
			--env HTTPS_PROXY=$(HTTPS_PROXY)                        \
			--env KUBECONFIG=$(KUBECONFIG)                          \
			--env SRC_CONTEXT=$(SRC_CONTEXT)                        \
			--env DST_CONTEXT=$(DST_CONTEXT)                        \
			$(BUILD_IMAGE)                                          \
			/bin/bash -c "                                          \
				DOCKER_REGISTRY=$(REGISTRY)                         \
				KUBECONFIG=$${KUBECONFIG#$(HOME)}                   \
				./hack/reset.sh		                                \
				"
