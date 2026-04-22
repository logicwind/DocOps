BINARY := docops
PKG    := github.com/logicwind/docops
VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
  -X $(PKG)/internal/version.Version=$(VERSION) \
  -X $(PKG)/internal/version.Commit=$(COMMIT) \
  -X $(PKG)/internal/version.Date=$(DATE)

.PHONY: build install test lint clean tidy release-snapshot

build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/docops

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/docops

test:
	go test -race ./...

lint:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin dist

release-snapshot:
	goreleaser release --snapshot --clean
