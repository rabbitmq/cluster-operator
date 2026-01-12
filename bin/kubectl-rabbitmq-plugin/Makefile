
PLUGIN_NAME := kubectl-rabbitmq
PLUGIN_VERSION ?= dev

DEFAULT_GOAL := build
SOURCES := $(wildcard *.go)

.PHONY: build
build::$(PLUGIN_NAME)

$(PLUGIN_NAME): *.go go.mod go.sum
	go build -o $(PLUGIN_NAME) -ldflags "-X main.pluginVersion=$(PLUGIN_VERSION)" $(filter-out %_test.go,$(SOURCES))

.PHONY: clean
clean:
	rm -f $(PLUGIN_NAME)

.PHONY: tests
tests: build
	go test -tags "$(TEST_TAGS)" -cover -parallel 4 -shuffle on -v -coverprofile cover.out .
