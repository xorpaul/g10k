DEPS = $(wildcard */*.go)
BUILDVERSION = $(shell git describe --tags)
BUILDTIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
UNAME := $(shell uname)

all: test g10k

g10k: g10k.go $(DEPS)
# -race flag is currently removed because of issues in OS X Monterey. Should be solved above go version 1.17.6
ifeq ($(UNAME), Darwin)
	GO111MODULE=on CGO_ENABLED=1 GOOS=darwin go build \
		-ldflags "-s -w -X main.buildversion=${BUILDVERSION} -X main.buildtime=${BUILDTIME}" \
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
	goimports -d *.go tests/

test: lint vet imports
# This is a workaround for Bug https://github.com/golang/go/issues/49138
ifeq ($(UNAME), Darwin)
	GO111MODULE=on MallocNanoZone=0 go test -race -coverprofile=coverage.txt -covermode=atomic -v
endif
ifeq ($(UNAME), Linux)
	GO111MODULE=on go test -race -coverprofile=coverage.txt -covermode=atomic -v
endif

clean:
	rm -rf g10k coverage.txt cache example

build-image:
	docker build -t g10k:${BUILDVERSION} .

update-deps:
	GO111MODULE=on go get -u
	GO111MODULE=on go mod vendor

.PHONY: all lint vet imports test clean
