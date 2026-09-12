# This Makefile provides targets for building, linting and testing.

NAME = nerdqaxe-exporter

# Build informations
BIN = ${BUILD_DIR}/${NAME}.${GOARCH}
BUILD_DIR := build
DOCKER_FILE := docker/Dockerfile
DOCKER_IMAGE := docker.io/theo01/${NAME}:latest
TARGET ?= http://192.0.2.10
GO_MAIN := ./main.go
GOARCH ?= $(shell go env GOARCH)
LDFLAGS := -s -w \
	-X github.com/prometheus/common/version.Version=$(shell git describe --always --tags) \
	-X github.com/prometheus/common/version.Revision=$(shell git rev-parse HEAD) \
	-X github.com/prometheus/common/version.Branch=$(shell git rev-parse --abbrev-ref HEAD) \
	-X github.com/prometheus/common/version.BuildUser=$(shell whoami)@$(shell hostname) \
	-X github.com/prometheus/common/version.BuildDate=$(shell date --utc +%FT%T)
GOOS = linux

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
	mkdir -p ${BUILD_DIR}
	CGO_ENABLED=1 GOOS=${GOOS} GOARCH=${GOARCH} \
  go build -v -o ${BIN} -ldflags=" \
  ${LDFLAGS}" \
  ${GO_MAIN}

build-amd64: GOARCH = amd64
build-amd64: build ## Build the binary for AMD64 Linux

build-arm64: GOARCH = arm64
build-arm64: build ## Build the binary for ARM64 Linux

docker: ## Build the Docker image
	docker build --platform ${GOOS}/${GOARCH} -f $(DOCKER_FILE) -t $(DOCKER_IMAGE) .

docker-arm64: GOARCH = arm64
docker-arm64: docker ## Build the Docker image for ARM64

docker-amd64: GOARCH = amd64
docker-amd64: docker ## Build the Docker image for AMD64

docker-all: ## Build the Docker image for all architectures
	docker buildx build --platform linux/amd64,linux/arm64 -f $(DOCKER_FILE) -t $(DOCKER_IMAGE) .

docker-podman: docker
	skopeo copy docker-daemon:${DOCKER_IMAGE} containers-storage:${DOCKER_IMAGE}

clean: ## Clean build artifacts
	-@rm -rf ${BUILD_DIR}

## @Development

run: build ## Run the exporter
	${BIN} --target ${TARGET}

run-docker: docker ## Run the bot in Docker container
	docker run --rm -it  \
		-v $(PWD)/config.yaml:/app/config/config.yaml \
		-v $(PWD)/config.env:/app/config/config.env \
		$(DOCKER_IMAGE) \
		bot --config /app/config/config.yaml --config-env /app/config/config.env --log-level=debug

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
