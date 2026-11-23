FROM golang:1.27-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w main.version=${VERSION}" \
    -o /out/alerthub-mcp cmd/main.go

FROM alpine:3.21 AS certs
RUN apk --no-cache add ca-certificates

FROM scratch

ARG VERSION=dev

LABEL io.modelcontextprotocol.server.name="io.github.alerthubio/mcp-server" \
      org.opencontainers.image.title="AlertHub MCP Server" \
      org.opencontainers.image.description="Send instant and scheduled multi-channel alerts via AlertHub" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/alerthubio/mcp-server" \
      org.opencontainers.image.licenses="MIT"

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/alerthub-mcp /alerthub-mcp

USER 65534:65534

ENTRYPOINT ["/alerthub-mcp"]
