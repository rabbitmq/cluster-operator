
GO_OS = $(shell go env GOOS)
GO_ARCH = $(shell go env GOARCH)
PLUGIN_VERSION ?= dev
PLUGIN_NAME := kubectl-rabbitmq-$(PLUGIN_VERSION)-$(GO_OS)-$(GO_ARCH)

DEFAULT_GOAL := build
SOURCES := $(wildcard *.go)

.PHONY: build
build::$(PLUGIN_NAME)

# Omit debug symbols and DWARF table for non-dev builds
ifneq ($(PLUGIN_VERSION),dev)
GO_BUILD_FLAGS := -s -w
endif

$(PLUGIN_NAME): *.go go.mod go.sum
	go build -o $(PLUGIN_NAME) -ldflags "-X main.pluginVersion=$(PLUGIN_VERSION) $(GO_BUILD_FLAGS)" $(filter-out %_test.go,$(SOURCES))

.PHONY: clean
clean:
	rm -f $(PLUGIN_NAME)

ifeq ($(CI),)
TEST_CI_FLAGS := -coverprofile cover.out
endif

.PHONY: tests
tests:
	go test -tags "$(TEST_TAGS)" -cover -parallel 4 -shuffle on -v $(TEST_CI_FLAGS) $(TEST_FLAGS) .
