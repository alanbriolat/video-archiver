UNAME := $(shell uname -o)

ifeq ($(UNAME), Msys)
MINGW64_ROOT ?= /mingw64
MINGW64_BIN ?= $(MINGW64_ROOT)/bin
GOROOT ?= $(MINGW64_ROOT)/lib/go
endif

.PHONY: all
all: build

.PHONY: build
build:
	go build -v ./...
	go build -v -o ./bin/ ./cmd/download-video
	go build -v -o ./bin/ ./cmd/video-archiver

.PHONY: build-windows
build-windows:
	$(MAKE) build PATH="$(MINGW64_BIN):$(PATH)" GOROOT="$(GOROOT)"

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	go clean ./...
