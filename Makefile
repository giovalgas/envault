BINARY := envault
PKG := ./cmd/envault
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
FUZZTIME ?= 30s
SKILL_CONTENT := internal/skill/domain/content.go

.PHONY: build test lint fuzz cover release-snapshot skill-sync skill-check

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

test:
	go test -race ./...

lint:
	golangci-lint run ./...

fuzz:
	go test -run='^$$' -fuzz=FuzzParse -fuzztime=$(FUZZTIME) ./internal/shared/dotenv

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

release-snapshot:
	goreleaser release --snapshot --clean

skill-sync:
	go run ./tools/skillgen

skill-check: skill-sync
	git diff --exit-code -- $(SKILL_CONTENT)
