BINARY := gitlab-pipelines
PKG := ./cmd/gitlab-pipelines/

.PHONY: build clean test lint

build:
	go build -o $(BINARY) $(PKG)

clean:
	rm -f $(BINARY)

test:
	go test ./...

lint:
	golangci-lint run
