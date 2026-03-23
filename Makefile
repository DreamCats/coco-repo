APP_NAME := coco-repo
VERSION := v0.1.0
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +%Y-%m-%d)
LDFLAGS := -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build build-all test clean install

build:
	go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) .

build-all:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_darwin_amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_darwin_arm64 .
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_linux_amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_linux_arm64 .

test:
	go test ./... -v

clean:
	rm -f $(APP_NAME)
	rm -rf dist/

install: build
	@mkdir -p $(HOME)/.local/bin
	mv $(APP_NAME) $(HOME)/.local/bin/
