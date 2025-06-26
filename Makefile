.PHONY: proto-format proto-lint proto-gen license format lint build run
all: proto-all format lint test-unit build

###############################################################################
###                                  Build                                  ###
###############################################################################

build:
	@echo "Building simd binary..."
	@cd simapp && make build > /dev/null
	@echo "Build completed successfully."

run: build
	@./local.sh

###############################################################################
###                                 Tooling                                 ###
###############################################################################

gofumpt_cmd=mvdan.cc/gofumpt
goimports_reviser_cmd=github.com/incu6us/goimports-reviser/v3
golangci_lint_cmd=github.com/golangci/golangci-lint/cmd/golangci-lint

FILES := $(shell find . -name "*.go" -not -path "./e2e/*" -not -path "./simapp/*" -not -name "*.pb.go" -not -name "*.pb.gw.go" -not -name "*.pulsar.go")

license:
	@echo "Verifying and applying license headers..."
	@go-license --config .github/license.yml $(FILES)
	@echo "License headers updated."

format:
	@echo "Formatting Go source files..."
	@go run $(gofumpt_cmd) -l -w keeper/*
	@go run $(goimports_reviser_cmd) keeper/* > /dev/null
	@echo "Formatting complete."

lint:
	@echo "Running Go linter..."
	@go run $(golangci_lint_cmd) run --timeout=10m
	@echo "Linting complete."

###############################################################################
###                                Protobuf                                 ###
###############################################################################

BUF_VERSION=1.50
BUILDER_VERSION=0.15.3

proto-all: proto-format proto-lint proto-gen

proto-format:
	@echo "Formatting protobuf definitions..."
	@docker run --rm --volume "$(PWD)":/workspace --workdir /workspace \
		bufbuild/buf:$(BUF_VERSION) format --diff --write
	@echo "Protobuf formatting done."

proto-gen:
	@echo "Generating Go code from protobuf files..."
	@docker run --rm --volume "$(PWD)":/workspace --workdir /workspace \
		ghcr.io/cosmos/proto-builder:$(BUILDER_VERSION) sh ./proto/generate.sh
	@echo "Protobuf code generation done."

proto-lint:
	@echo "Linting protobuf files..."
	@docker run --rm --volume "$(PWD)":/workspace --workdir /workspace \
		bufbuild/buf:$(BUF_VERSION) lint
	@echo "Protobuf linting complete."

###############################################################################
###                                 Testing                                 ###
###############################################################################

test-unit:
	@echo "Running unit tests with coverage..."
	@go test -cover -coverpkg=./keeper/... -coverprofile=coverage.out -race -v ./keeper/...
	@go tool cover -html=coverage.out && go tool cover -func=coverage.out
	@echo "Unit tests completed."
