PLUGIN_NAME = hgarfer/sshfs
PLUGIN_TAG ?= next

.PHONY: all clean rootfs create enable push test test-unit test-integration test-coverage lint fmt vet help

all: clean rootfs create

clean:
	@echo "### rm ./plugin"
	@rm -rf ./plugin

rootfs:
	@echo "### docker build: rootfs image with docker-volume-sshfs"
	@docker build -q -t ${PLUGIN_NAME}:rootfs .
	@echo "### create rootfs directory in ./plugin/rootfs"
	@mkdir -p ./plugin/rootfs
	@docker create --name tmp ${PLUGIN_NAME}:rootfs
	@docker export tmp | tar -x -C ./plugin/rootfs
	@echo "### copy config.json to ./plugin/"
	@cp config.json ./plugin/
	@docker rm -vf tmp

create:
	@echo "### remove existing plugin ${PLUGIN_NAME}:${PLUGIN_TAG} if exists"
	@docker plugin rm -f ${PLUGIN_NAME}:${PLUGIN_TAG} || true
	@echo "### create new plugin ${PLUGIN_NAME}:${PLUGIN_TAG} from ./plugin"
	@docker plugin create ${PLUGIN_NAME}:${PLUGIN_TAG} ./plugin

enable:		
	@echo "### enable plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"		
	@docker plugin enable ${PLUGIN_NAME}:${PLUGIN_TAG}

push:  clean rootfs create enable
	@echo "### push plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	@docker plugin push ${PLUGIN_NAME}:${PLUGIN_TAG}

# Test targets
test: test-unit  ## Run all tests (unit tests only by default)
	@echo "### All tests passed!"

test-unit:  ## Run unit tests
	@echo "### Running unit tests..."
	@go test -v -race -timeout 30s -coverprofile=coverage-unit.out ./...

test-integration:  ## Run integration tests (requires Docker and SSH server)
	@echo "### Running integration tests..."
	@echo "### Note: Integration tests require Docker and will start an SSH container"
	@INTEGRATION_TESTS=1 go test -v -race -timeout 5m -tags=integration -coverprofile=coverage-integration.out ./...

test-coverage:  ## Run tests with coverage report
	@echo "### Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "### Coverage report generated: coverage.html"

test-coverage-full: test-unit test-integration  ## Run all tests with full coverage
	@echo "### Merging coverage reports..."
	@echo "mode: atomic" > coverage-full.out
	@tail -n +2 coverage-unit.out >> coverage-full.out
	@if [ -f coverage-integration.out ]; then tail -n +2 coverage-integration.out >> coverage-full.out; fi
	@go tool cover -html=coverage-full.out -o coverage-full.html
	@echo "### Full coverage report generated: coverage-full.html"

# Code quality targets
lint:  ## Run linter (golangci-lint)
	@echo "### Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

fmt:  ## Format code with gofmt
	@echo "### Formatting code..."
	@gofmt -s -w .
	@echo "### Code formatted"

fmt-check:  ## Check if code is formatted
	@echo "### Checking code formatting..."
	@if [ -n "$$(gofmt -s -l . | grep -v vendor)" ]; then \
		echo "The following files are not formatted:"; \
		gofmt -s -l . | grep -v vendor; \
		exit 1; \
	fi
	@echo "### Code is properly formatted"

vet:  ## Run go vet
	@echo "### Running go vet..."
	@go vet ./...
	@echo "### go vet passed"

# CI targets (mimicking old Travis tests)
ci-unit: fmt-check vet test-unit  ## Run CI unit tests (format check + vet + unit tests)
	@echo "### CI unit tests passed!"

ci-integration: test-integration  ## Run CI integration tests
	@echo "### CI integration tests passed!"

# Dependency management
deps:  ## Download dependencies
	@echo "### Downloading dependencies..."
	@go mod download
	@echo "### Dependencies downloaded"

tidy:  ## Tidy go.mod
	@echo "### Tidying go.mod..."
	@go mod tidy
	@echo "### go.mod tidied"

vendor:  ## Vendor dependencies
	@echo "### Vendoring dependencies..."
	@go mod vendor
	@echo "### Dependencies vendored"

# Build targets
build:  ## Build the binary
	@echo "### Building binary..."
	@go build -o docker-volume-sshfs
	@echo "### Binary built: docker-volume-sshfs"

install:  ## Install the binary
	@echo "### Installing binary..."
	@go install
	@echo "### Binary installed"

# Help target
help:  ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
