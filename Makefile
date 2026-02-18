.PHONY: build test run clean

build:
	go build -o alsamixer-web ./cmd/alsamixer-web

test:
	go test ./...

run:
	go run ./cmd/alsamixer-web

clean:
	rm -f alsamixer-web
