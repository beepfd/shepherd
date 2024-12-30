GO := go
GO_BUILD = CGO_ENABLED=1 $(GO) build
GO_GENERATE = $(GO) generate
GO_TAGS ?=
TARGET_GOARCH ?= amd64,arm64
GOARCH ?= amd64
GOOS ?= linux
VERSION=$(shell git describe --tags --always)
# For compiling libpcap and CGO
CC ?= gcc


build: elf
	cd ./cmd;CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_LDFLAGS='-g -lcapstone -static'   go build -tags=netgo,osusergo -gcflags "all=-N -l" -v  -o shepherd

dlv:  build
	dlv --headless --listen=:2345 --api-version=2 exec ./cmd/shepherd -- --config-path=./cmd/config.yaml

run:  build
	./cmd/shepherd --config-path=./cmd/config.yaml

elf:
	TARGET_GOARCH=$(TARGET_GOARCH) $(GO_GENERATE)
    	CC=$(CC) GOARCH=$(TARGET_GOARCH) $(GO_BUILD) $(if $(GO_TAGS),-tags $(GO_TAGS)) \
    		-ldflags "-w -s "

image:
	docker buildx create --use
	docker buildx build --platform linux/amd64 -t ghostbaby/shepherd:v0.0.1-amd64 --push .
	docker buildx build --platform linux/arm64 -t ghostbaby/shepherd:v0.0.1-arm64 --push .
