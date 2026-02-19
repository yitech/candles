# ── builder ────────────────────────────────────────────────────────────────────
FROM golang:1.23 AS builder

ENV GOFLAGS=-mod=mod

# Install protoc compiler
RUN apt-get update && \
    apt-get install -y --no-install-recommends protobuf-compiler && \
    rm -rf /var/lib/apt/lists/*

# Install protoc Go plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

WORKDIR /app

# Copy all sources
COPY . .

# Generate gRPC code then build binaries
RUN protoc \
      --go_out=proto --go_opt=paths=source_relative \
      --go-grpc_out=proto --go-grpc_opt=paths=source_relative \
      proto/candle.proto && \
    go mod tidy && \
    go build -o /bin/srv    ./cmd/srv && \
    go build -o /bin/client ./cmd/client

# ── final ──────────────────────────────────────────────────────────────────────
FROM alpine:latest AS final

COPY --from=builder /bin/srv    /bin/srv
COPY --from=builder /bin/client /bin/client

# Default entrypoint — override in docker-compose for the client binary
CMD ["/bin/srv"]
