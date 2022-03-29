UNAME := $(shell uname -o)

ifeq ($(UNAME), Msys)
MINGW64_ROOT ?= /mingw64
MINGW64_BIN ?= $(MINGW64_ROOT)/bin
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
	# Copy GTK theme and settings for selecting that theme
	mkdir -p ./bin/share/themes/Windows10/
	cp -r resources/themes/Windows-10/gtk-3.20 ./bin/share/themes/Windows10/gtk-3.0
	mkdir -p ./bin/share/icons/
	cp -r $(MINGW64_ROOT)/share/icons/Adwaita ./bin/share/icons/
	cp -r $(MINGW64_ROOT)/share/icons/hicolor ./bin/share/icons/
	mkdir -p ./bin/etc/gtk-3.0/
	cp resources/gtk-settings.ini ./bin/etc/gtk-3.0/settings.ini
	mkdir -p ./bin/share/glib-2.0/schemas
	$(MINGW64_BIN)/glib-compile-schemas ./bin/share/glib-2.0/schemas

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	go clean ./...
