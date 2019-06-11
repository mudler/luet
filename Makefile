NAME ?= luet
PACKAGE_NAME ?= $(NAME)
PACKAGE_CONFLICT ?= $(PACKAGE_NAME)-beta
REVISION := $(shell git rev-parse --short HEAD || echo unknown)
VERSION := $(shell git describe --tags || echo dev)
VERSION := $(shell echo $(VERSION) | sed -e 's/^v//g')
ITTERATION := $(shell date +%s)
BUILD_PLATFORMS ?= -osarch="linux/amd64" -osarch="linux/386" -osarch="linux/arm"

.PHONY: all
all: deps build

.PHONY: test
test:
	ginkgo -r ./...

.PHONY: coverage
coverage:
	go test ./... -race -coverprofile=coverage.txt -covermode=atomic

.PHONY: help
help:
	# make all => deps test lint build
	# make deps - install all dependencies
	# make test - run project tests
	# make lint - check project code style
	# make build - build project for all supported OSes

.PHONY: clean
clean:
	rm -rf release/

.PHONY: deps
deps:
	go env
	# Installing dependencies...
	go get -u golang.org/x/lint/golint
	go get github.com/mitchellh/gox
	go get golang.org/x/tools/cmd/cover
	go get github.com/onsi/ginkgo/ginkgo
	go get github.com/onsi/gomega/...

.PHONY: build
build:
	go build

.PHONY: gox-build
gox-build:
	# Building gitlab-ci-multi-runner for $(BUILD_PLATFORMS)
	gox $(BUILD_PLATFORMS) -output="release/$(NAME)-$(VERSION)-{{.OS}}-{{.Arch}}"

.PHONY: lint
lint:
	# Checking project code style...
	golint ./... | grep -v "be unexported"

