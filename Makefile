
# go tool nm ./luet | grep Commit
override LDFLAGS += -X "github.com/mudler/luet/cmd.BuildTime=$(shell date -u '+%Y-%m-%d %I:%M:%S %Z')"
override LDFLAGS += -X "github.com/mudler/luet/cmd.BuildCommit=$(shell git rev-parse HEAD)"

NAME ?= luet
PACKAGE_NAME ?= $(NAME)
PACKAGE_CONFLICT ?= $(PACKAGE_NAME)-beta
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

.PHONY: all
all: deps build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: test
test:
	GO111MODULE=off go get github.com/onsi/ginkgo/v2
	go install github.com/onsi/ginkgo/v2/ginkgo
	GO111MODULE=off go get github.com/onsi/gomega/...
	ginkgo -r --flake-attempts=3 ./...

.PHONY: test-integration
test-integration:
	tests/integration/run.sh

.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.txt -covermode=atomic

.PHONY: test-coverage
test-coverage:
	scripts/ginkgo.coverage.sh --codecov

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
	rm -rf tests/integration/shunit2
	rm -rf tests/integration/bin

.PHONY: deps
deps:
	go env
	# Installing dependencies...
	GO111MODULE=off go get golang.org/x/lint/golint
	GO111MODULE=off go get github.com/mitchellh/gox
	GO111MODULE=off go get golang.org/x/tools/cmd/cover
	GO111MODULE=off go get github.com/onsi/ginkgo/ginkgo
	GO111MODULE=off go get github.com/onsi/gomega/...

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'

.PHONY: build-small
build-small:
	@$(MAKE) LDFLAGS+="-s -w" build
	upx --brute -1 $(NAME)

.PHONY: image
image:
	docker build --rm -t luet/base .

.PHONY: lint
lint:
	golint ./... | grep -v "be unexported"

.PHONY: vendor
vendor:
	go mod vendor

.PHONY: test-docker
test-docker:
	docker run -v $(ROOT_DIR):/go/src/github.com/mudler/luet \
				--workdir /go/src/github.com/mudler/luet -ti golang:latest \
				bash -c "make test"

multiarch-build:
	goreleaser build --snapshot --rm-dist

multiarch-build-small:
	@$(MAKE) multiarch-build
	for file in $(ROOT_DIR)/release/**/* ; do \
		upx --brute -1 $${file} ; \
	done
