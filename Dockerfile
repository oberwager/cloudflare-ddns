FROM golang:1.25-alpine AS builder

ARG VERSION=dev
WORKDIR /build

COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a \
    -ldflags "-w -s -extldflags '-static' -X 'main.Version=${VERSION}'" \
    -o cloudflare-ddns .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/cloudflare-ddns /cloudflare-ddns

USER 65534:65534

ENTRYPOINT ["/cloudflare-ddns"]
