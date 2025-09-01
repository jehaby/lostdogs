FROM golang:1.25-bookworm AS builder

WORKDIR /src

ENV CGO_ENABLED=1

RUN apt-get update -y && apt-get install -y --no-install-recommends \
    build-essential ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -trimpath -o /out/lostdogs ./cmd/lostdogs

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update -y && apt-get install -y --no-install-recommends \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/lostdogs /app/lostdogs
COPY resources ./resources

VOLUME ["/data"]

ENV PATH="/app:${PATH}"

ENTRYPOINT ["/app/lostdogs"]
