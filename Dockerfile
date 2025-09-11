FROM golang:1.24 AS builder
WORKDIR /workspace

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOCACHE=/go-cache
ENV GOMODCACHE=/gomod-cache

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/gomod-cache go mod download

# Build layer
COPY . .
RUN --mount=type=cache,target=/gomod-cache \
    --mount=type=cache,target=/go-cache \
    go build -ldflags="-s -w" -o app .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /root/

COPY --from=builder /workspace/app .

ENTRYPOINT ["./app"]