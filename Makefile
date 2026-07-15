BINARY=media-cli
MODULE=radioplatform-media-ci

.PHONY: build test vet clean

build:
	go build -o $(BINARY) ./cmd/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
	go clean

all: vet test build

install:
	go install ./cmd/$(BINARY)
