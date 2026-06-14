# vim:ft=make:

.PHONY: all build clean update demo test lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY := ip-to-cloudprovider

all: test build

test:
	@NO_COLOR=true go test ./...

lint:
	@go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

build: clean
	@go fmt ./...
	@go build $(LDFLAGS) -o $(BINARY) .

update: build
	./$(BINARY) -a

demo: build
	./$(BINARY) scan-file demo_ips.txt

clean:
	@rm -f $(BINARY) $(BINARY).exe
