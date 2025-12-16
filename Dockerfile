# syntax=docker/dockerfile:1.7
ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata build-base
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/onessa ./cmd/

# --- runtime ---
FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S app && adduser -S app -G app \
  && mkdir -p /data /app \
  && chown -R app:app /data /app

WORKDIR /app
ENV DATA_DIR=/data HTTP_ADDR=:8080

COPY --from=builder /bin/onessa /app/onessa
COPY static /app/static

USER app
EXPOSE 8080
ENTRYPOINT ["/app/onessa"]