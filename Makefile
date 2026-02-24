.PHONY: test run clean alsamixer-web dist build-linux-arm64 build-linux-amd64 deploy

VERSION ?= $(shell git describe --tags --always --dirty --match "v*")
ifeq ($(VERSION),)
    VERSION = dev
endif

LDFLAGS = -ldflags "-s -w -X main.version=$(VERSION)"

alsamixer-web:
	go build $(LDFLAGS) -o alsamixer-web ./cmd/alsamixer-web

dist:
	mkdir -p dist

test:
	go test ./...

run:
	go run $(LDFLAGS) ./cmd/alsamixer-web

BUILDER_IMAGE ?= alsamixer-builder

builder-image:
	docker build --platform=linux/amd64 -f Dockerfile.builder -t $(BUILDER_IMAGE) .

build-linux-arm64: dist builder-image
	docker run --rm --platform=linux/arm64 -v "$(PWD)":/src -w /src $(BUILDER_IMAGE) \
		go build -ldflags "-s -w -X main.version=$(VERSION)" \
		-o dist/alsamixer-web-linux-arm64 ./cmd/alsamixer-web

build-linux-amd64: dist builder-image
	docker run --rm --platform=linux/amd64 -v "$(PWD)":/src -w /src $(BUILDER_IMAGE) \
		go build -ldflags "-s -w -X main.version=$(VERSION)" \
		-o dist/alsamixer-web-linux-amd64 ./cmd/alsamixer-web

DEPLOY_TARGET ?=
DEPLOY_PATH ?=

deploy: build-linux-amd64
	@if [ -z "$(DEPLOY_TARGET)" ] || [ -z "$(DEPLOY_PATH)" ]; then \
		echo "Error: DEPLOY_TARGET and DEPLOY_PATH must be set"; \
		echo "Usage: make deploy DEPLOY_TARGET=user@host DEPLOY_PATH=/path/to/dest"; \
		exit 1; \
	fi
	scp dist/alsamixer-web-linux-amd64 $(DEPLOY_TARGET):$(DEPLOY_PATH)/alsamixer-web.new
	ssh $(DEPLOY_TARGET) "mv $(DEPLOY_PATH)/alsamixer-web.new $(DEPLOY_PATH)/alsamixer-web && chmod +x $(DEPLOY_PATH)/alsamixer-web"

clean:
	rm -rf alsamixer-web dist/
