# This Makefile provides targets for building, linting and testing.

NAME = nerdqaxe-exporter

# Build informations
BIN = ${BIN_DIR}/${NAME}
BIN_DIR = ${BUILD_DIR}/${GOOS}/${GOARCH}/bin
BUILD_DIR := build
DOCKER_FILE := docker/Dockerfile
DOCKER_IMAGE := docker.io/theo01/${NAME}:latest
TARGET ?= http://nerdqaxe.local
GO_MAIN := .
GOARCH ?= $(shell go env GOARCH)
LDFLAGS := -s -w \
	-X github.com/prometheus/common/version.Version=$(shell git describe --always --tags) \
	-X github.com/prometheus/common/version.Revision=$(shell git rev-parse HEAD) \
	-X github.com/prometheus/common/version.Branch=$(shell git rev-parse --abbrev-ref HEAD) \
	-X github.com/prometheus/common/version.BuildUser=$(shell whoami)@$(shell hostname) \
	-X github.com/prometheus/common/version.BuildDate=$(shell date --utc +%FT%T)
GOOS ?= $(shell go env GOOS)

# Makefile targets
.PHONY: build build-amd64 build-arm64 docker docker-amd64 docker-arm64 docker-all clean run install test test-concurrency lint golangci-lint go-lint vet fmt security nancy help
.DEFAULT_GOAL := build

# Colors for output
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

##@ Development environment

setup: ## Setup the development environment
	pre-commit install

##@ Build

build: ## Build the binary
	mkdir -p ${BIN_DIR}
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} \
  go build -v -o ${BIN} -ldflags=" \
  ${LDFLAGS}" \
  ${GO_MAIN}

build-amd64: ## Build the binary for AMD64 Linux
	$(MAKE) build GOARCH=amd64

build-arm64: ## Build the binary for ARM64 Linux
	$(MAKE) build GOARCH=arm64

# The Dockerfile takes the binary from the build directory, laid out as
# <os>/<arch>/bin/<binary>, the same layout GoReleaser passes as context.
docker: build ## Build the Docker image
	docker build --platform ${GOOS}/${GOARCH} -f $(DOCKER_FILE) -t $(DOCKER_IMAGE) ${BUILD_DIR}

docker-arm64: ## Build the Docker image for ARM64
	$(MAKE) docker GOARCH=arm64

docker-amd64: ## Build the Docker image for AMD64
	$(MAKE) docker GOARCH=amd64

docker-all: build-amd64 build-arm64 ## Build the Docker image for all architectures
	docker buildx build --platform linux/amd64,linux/arm64 -f $(DOCKER_FILE) -t $(DOCKER_IMAGE) ${BUILD_DIR}

docker-podman: docker
	skopeo copy docker-daemon:${DOCKER_IMAGE} containers-storage:${DOCKER_IMAGE}

clean: ## Clean build artifacts
	-@rm -rf ${BUILD_DIR}

## @Development

run: build ## Run the exporter
	${BIN} --target ${TARGET}

run-docker: docker ## Run the exporter in Docker container
	docker run --rm -it -p 10055:10055 $(DOCKER_IMAGE) --target ${TARGET}

##@ Install the binary

install: build ## Install the binary to ~/.local/bin
	@mkdir -p ~/.local/bin
	cp ${BIN} ~/.local/bin/${NAME}

##@ Testing

test: ## Run all tests
	go test -v ./...

test-concurrency: ## Run tests with concurrency tag
	go test -v --tags=concurrency ./...

##@ Code Quality

lint: golangci-lint go-lint vet fmt  ## Run all linters

fmt: ## Run gofmt
	gofmt -d .

golangci-lint: ## Run golangci-lint
	golangci-lint run -E gosec -E goconst --timeout 10m --max-same-issues 0 --max-issues-per-linter 0 ./...

go-lint: ## Run golint
	golint ./...

vet: ## Run go vet
	go vet ./...

##@ Security

security: nancy ## Run all security scans

nancy: ## Run Nancy vulnerability scan
	sh -c "go list -json -m all | nancy sleuth"

##@ Help

help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(CYAN)Usage:$(RESET)\n  make $(YELLOW)<target>$(RESET)\n"} /^[a-zA-Z_0-9-]+.*?##/ { printf "  $(YELLOW)%-20s$(RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(CYAN)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
