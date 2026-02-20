.PHONY: test run clean alsamixer-web dist build-linux-arm64 build-linux-amd64 deploy-lemox

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

deploy-lemox:
	$(MAKE) build-linux-amd64
	scp dist/alsamixer-web-linux-amd64 root@lemox.lan:/root/work/alsamixer-web/alsamixer-web
	ssh root@lemox.lan "chmod +x /root/work/alsamixer-web/alsamixer-web"

clean:
	rm -rf alsamixer-web dist/
