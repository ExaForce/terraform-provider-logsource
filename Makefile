BINARY     = terraform-provider-logsource
INSTALL_DIR = ~/.terraform.d/plugins/registry.terraform.io/exaforce/logsource/1.0.0/darwin_arm64

.PHONY: build install fmt lint tidy

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/

fmt:
	gofmt -w .

lint:
	go vet ./...

tidy:
	go mod tidy
