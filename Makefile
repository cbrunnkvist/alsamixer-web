.PHONY: test run clean alsamixer-web dist build-linux-arm64 build-linux-amd64 deploy install-service

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

DEPLOY_TARGET ?= root@lemox.lan
DEPLOY_PATH ?= /root/work/alsamixer-web

deploy: build-linux-amd64
	@if [ -z "$(DEPLOY_TARGET)" ] || [ -z "$(DEPLOY_PATH)" ]; then \
		echo "Error: DEPLOY_TARGET and DEPLOY_PATH must be set"; \
		echo "Usage: make deploy DEPLOY_TARGET=user@host DEPLOY_PATH=/path/to/dest"; \
		exit 1; \
	fi
	scp dist/alsamixer-web-linux-amd64 $(DEPLOY_TARGET):$(DEPLOY_PATH)/alsamixer-web.new
	ssh $(DEPLOY_TARGET) "mv $(DEPLOY_PATH)/alsamixer-web.new $(DEPLOY_PATH)/alsamixer-web && chmod +x $(DEPLOY_PATH)/alsamixer-web"

install-service:
	@if [ -z "$(DEPLOY_TARGET)" ] || [ -z "$(DEPLOY_PATH)" ]; then \
		echo "Error: DEPLOY_TARGET and DEPLOY_PATH must be set"; \
		exit 1; \
	fi
	ssh $(DEPLOY_TARGET) "mkdir -p $(DEPLOY_PATH) /root/.config/systemd/user"
	scp deploy/alsamixer-web.service $(DEPLOY_TARGET):$(DEPLOY_PATH)/alsamixer-web.service
	scp alsamixer-web-wrapper $(DEPLOY_TARGET):$(DEPLOY_PATH)/alsamixer-web-wrapper
	ssh $(DEPLOY_TARGET) "chmod +x $(DEPLOY_PATH)/alsamixer-web-wrapper"
	ssh $(DEPLOY_TARGET) "ln -sf $(DEPLOY_PATH)/alsamixer-web.service /root/.config/systemd/user/alsamixer-web.service"
	ssh $(DEPLOY_TARGET) "systemctl --user daemon-reload && systemctl --user enable alsamixer-web.service && systemctl --user restart alsamixer-web.service"

clean:
	rm -rf alsamixer-web dist/
