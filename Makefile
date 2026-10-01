BINARY  := sitemap-audit
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
GOLANGCI_VERSION := v2.14.0

.PHONY: build install test race cover fuzz lint vuln docs cross release-check ready clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY) ./cmd/sitemap-audit

install:
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/sitemap-audit

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./internal/...
	@go tool cover -func=coverage.out | awk '/^total:/ { sub("%", "", $$3); print "coverage: " $$3 "%"; exit ($$3 >= 85) ? 0 : 1 }'

fuzz:
	go test -run '^$$' -fuzz FuzzParse -fuzztime 30s ./internal/robots
	go test -run '^$$' -fuzz FuzzParse -fuzztime 30s ./internal/sitemap

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

docs:
	go run ./cmd/sitemap-audit checks --markdown > docs/issues.md

cross:
	@for t in $(TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-$$os-$$arch$$ext ./cmd/sitemap-audit || exit 1; \
		echo "built dist/$(BINARY)-$$os-$$arch$$ext"; \
	done

release-check:
	go run github.com/goreleaser/goreleaser/v2@v2.18.2 check

ready: docs lint race cover vuln cross

clean:
	rm -rf dist coverage.out
