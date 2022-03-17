.PHONY: all
all: build

.PHONY: build
build:
	go build ./...

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	go clean ./...
