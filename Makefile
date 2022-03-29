UNAME := $(shell uname -o)

ifeq ($(UNAME), Msys)
MINGW64_BIN ?= /mingw64/bin
GOROOT ?= /c/msys64/mingw64/lib/go
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
	# Copy dependency DLLs
	PATH="./bin:$(MINGW64_BIN):$(PATH)" ldd ./bin/video-archiver.exe | grep "$(MINGW64_BIN)" | cut -f 3 -d ' ' | xargs -r cp -v -t ./bin/

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	go clean ./...
