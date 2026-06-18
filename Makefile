.PHONY: test race lint fmt tidy bench build release

BINARY ?= gomock
OUTPUT_DIR ?= bin
HOST_GOOS := $(shell go env GOOS)
HOST_GOARCH := $(shell go env GOARCH)
GOOS ?= $(HOST_GOOS)
GOARCH ?= $(HOST_GOARCH)
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
BINARY_EXT := $(if $(filter windows,$(GOOS)),.exe,)

ifeq ($(GOOS)/$(GOARCH),$(HOST_GOOS)/$(HOST_GOARCH))
OUTPUT ?= $(OUTPUT_DIR)/$(BINARY)$(BINARY_EXT)
else
OUTPUT ?= $(OUTPUT_DIR)/$(BINARY)_$(GOOS)_$(GOARCH)$(BINARY_EXT)
endif

test:
	go test ./...

race:
	go test -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w cmd internal test

tidy:
	go mod tidy

bench:
	go test -bench=. -benchmem ./...

build:
	mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath \
		-ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)" \
		-o $(OUTPUT) ./cmd/gomock

release:
	./scripts/build-release.sh
