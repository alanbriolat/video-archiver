.PHONY: all
all: build

.PHONY: build
build:
	go build -v ./...
	go build -v -o ./bin/ ./cmd/download-video
	go build -v -o ./bin/ ./cmd/video-archiver

.PHONY: build-windows
build-windows:
	$(MAKE) build PATH="/mingw54/bin:$(PATH)" GOROOT="/c/msys64/mingw64/lib/go"

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	go clean ./...
