# ============================================================
# Stage 1: Build
# ============================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /otacon ./cmd/otacon/

# ============================================================
# Stage 2: Runtime (distroless for minimal attack surface)
# ============================================================
FROM gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.title="Otacon"
LABEL org.opencontainers.image.description="Intelligent Kubernetes Diagnostics & Audit Platform"
LABEL org.opencontainers.image.source="https://github.com/merthan/otacon"
LABEL org.opencontainers.image.licenses="Apache-2.0"

COPY --from=builder /otacon /otacon
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER nonroot:nonroot

ENTRYPOINT ["/otacon"]
CMD ["guardian", "start"]
