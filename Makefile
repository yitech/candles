PROTO_DIR := proto
BIN_DIR   := bin

# Ensure Go-installed binaries (protoc-gen-go, protoc-gen-go-grpc) are on PATH
export PATH := $(shell go env GOPATH)/bin:$(PATH)

.PHONY: all proto deps build clean

all: build

## proto: generate Go code from proto/candle.proto
MODEL_DIR := model/protobuf

proto:
	mkdir -p $(MODEL_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(MODEL_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(MODEL_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/candle.proto

## deps: tidy and download Go modules (run after proto)
deps:
	go mod tidy

## build: generate proto, tidy deps, compile srv and client
build: proto deps
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/srv    ./cmd/srv
	go build -o $(BIN_DIR)/client ./cmd/client

## clean: remove compiled binaries
clean:
	rm -rf $(BIN_DIR)
