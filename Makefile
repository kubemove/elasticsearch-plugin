SHELL=/bin/bash -o pipefail

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
PLUGIN_IMAGE:=$(REGISTRY)/elasticsearch-plugin:$(IMAGE_TAG)
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

.PHONY: prepare
prepare:

.PHONY: install-plugin
install-plugin:

.PHONY: deploy-es
deploy-es:

.PHONY: setup-sync
setup-sync:

.PHONY: trigger-init
trigger-init:

.PHONY: trigger-sync
trigger-sync:

.PHONY: insert-index
insert-index:

.PHONY: show-indexes
show-indexes:
