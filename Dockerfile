# syntax=docker/dockerfile:1.7
FROM golang:1.25-bookworm AS builder

WORKDIR /src

ENV CGO_ENABLED=1

RUN apt-get update -y && apt-get install -y --no-install-recommends \
    build-essential ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
# Cache Go module downloads between builds
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Cache compiled objects between builds; speeds up incremental builds
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -o /out/lostdogs ./cmd/lostdogs

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update -y && apt-get install -y --no-install-recommends \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/lostdogs /app/lostdogs
COPY ./resources/db/migrations ./resources/db/migrations

VOLUME ["/data"]

ENV PATH="/app:${PATH}"

ENTRYPOINT ["/app/lostdogs"]
