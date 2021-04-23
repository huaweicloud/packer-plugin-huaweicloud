BINARY = packer-builder-huaweicloud-ecs
PLUGIN_DIR = ~/.packer.d/plugins
PLUGIN_FILE = ${PLUGIN_DIR}/${BINARY}
GOBIN = $(shell go env GOPATH)/bin
BINARY_FILE = ${GOBIN}/${BINARY}
GORELEASER_VER = 0.110.0
GOLANGCI_LINT_VER = 1.17.1
TEST?=$(shell go list ./...)

.PHONY: default
default: build test install

.PHONY: build
build:
	go install

.PHONY: install
install: build
	mkdir -p ${PLUGIN_DIR}
	@if [ -f ${PLUGIN_FILE} ]; then\
		rm ${PLUGIN_FILE};\
	fi
	ln -s ${BINARY_FILE} ${PLUGIN_FILE}

.PHONY: test
test:
	go test -v $(TEST) -timeout=3m

.PHONY: lint
lint:
	golint . ./huaweicloud
	golangci-lint run --skip-dirs=test,vendor --fast ./...

vet:
	@echo "go vet ."
	@go vet $$(go list ./... | grep -v vendor/) ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

.PHONY: clean
clean:
	rm -rf ${BINARY_FILE} ${PLUGIN_FILE}

.PHONY: setup-tools
setup-tools:
	# we want that `go get` install utilities, but in the module mode its
	# behaviour is different; actually, `go get` would rather modify the
	# local `go.mod`, so let's disable modules here.
	GO111MODULE=off go get -u golang.org/x/lint/golint
	GO111MODULE=off go get -u golang.org/x/tools/cmd/goimports
	# goreleaser and golangci-lint take pretty long to build
	# as an optimization, let's just download the binaries
	curl -sL "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VER}/goreleaser_Linux_x86_64.tar.gz" | tar -xzf - -C ${GOBIN} goreleaser
	curl -sL "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VER}/golangci-lint-${GOLANGCI_LINT_VER}-linux-amd64.tar.gz" | tar -xzf - -C ${GOBIN} --strip-components=1 "golangci-lint-${GOLANGCI_LINT_VER}-linux-amd64/golangci-lint"
