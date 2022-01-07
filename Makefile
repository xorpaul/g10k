DEPS = $(wildcard */*.go)
BUILDVERSION = $(shell git describe --tags)
BUILDTIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
UNAME := $(shell uname)

all: test g10k

g10k: g10k.go $(DEPS)
ifeq ($(UNAME), Darwin)
	GO111MODULE=on CGO_ENABLED=1 GOOS=darwin go build \
		-race -ldflags "-s -w -X main.buildversion=${BUILDVERSION} -X main.buildtime=${BUILDTIME}" \
	-o $@
	strip -X $@
endif
ifeq ($(UNAME), Linux)
	GO111MODULE=on CGO_ENABLED=1 GOOS=linux go build \
		-race -ldflags "-s -w -X main.buildversion=${BUILDVERSION} -X main.buildtime=${BUILDTIME}" \
	-o $@
	strip $@
endif

lint:
	GO111MODULE=on go install golang.org/x/lint/golint@latest && \
	golint *.go

vet: g10k.go
	GO111MODULE=on go vet

imports: g10k.go
	GO111MODULE=on go install golang.org/x/tools/cmd/goimports@latest && \
	goimports -d .

test: lint vet imports
	GO111MODULE=on go test -race -coverprofile=coverage.txt -covermode=atomic -v

clean:
	rm -f g10k coverage.txt

build-image:
	docker build -t g10k:${BUILDVERSION} .

.PHONY: all lint vet imports test clean
