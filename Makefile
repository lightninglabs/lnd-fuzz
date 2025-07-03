.PHONY: all build test clean

all: build

build:
	@mkdir -p bin
	go build -o bin/corpus-merge ./cmd/corpus-merge
	go build -o bin/cov-profiles ./cmd/cov-profiles
	go build -o bin/profile-diffs ./cmd/profile-diffs

test:
	go test -v ./...

test-coverage:
	go test -v -cover ./...

clean:
	rm -rf bin/
	rm -rf coverage/

install: build
	@echo "Tools built in ./bin/"
	@echo "Add $(PWD)/bin to your PATH to use them globally"