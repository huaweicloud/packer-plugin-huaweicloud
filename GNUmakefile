NAME=huaweicloud
BINARY=packer-plugin-${NAME}
PLUGIN_DIR = ~/.packer.d/plugins
PLUGIN_FILE = ${PLUGIN_DIR}/${BINARY}

COUNT?=1
TEST?=$(shell go list ./...)

.PHONY: install

build:
	go build -o ${BINARY}

install: build
	@mkdir -p ${PLUGIN_DIR}
	mv ${BINARY} ${PLUGIN_FILE}

run-example: install
	@packer build ./example

vet:
	@echo "go vet ."
	@go vet $$(go list ./...) ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

test:
	go test -v -count $(COUNT) $(TEST) -timeout=3m

testacc: install
	@PACKER_ACC=1 go test -count $(COUNT) -v $(TEST) -timeout=120m

.PHONY: clean
clean:
	rm -rf ${BINARY} ${PLUGIN_FILE}

install-gen-deps: ## Install dependencies for code generation
	@go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@latest

generate: ## install-gen-deps
	# add $GOPATH into $PATH when failed
	@go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@latest
	@go generate -v ./...

ci-release-docs:
	@go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@latest
	@packer-sdc renderdocs -src docs -partials docs-partials/ -dst docs/
	@/bin/sh -c "[ -d docs ] && zip -r docs.zip docs/"