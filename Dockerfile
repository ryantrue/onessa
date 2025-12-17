# syntax=docker/dockerfile:1

# Можно задавать версию Go при сборке (по умолчанию 1.25 — как в исходнике)
ARG GO_VERSION=1.25

# ==========================
# Этап сборки backend
# ==========================
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata build-base

WORKDIR /src

# Сначала зависимости — для кеша
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Остальной код
COPY . .

# Сборка бинарника из main в ./cmd
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/onessa ./cmd/


# =====================================================
# (Опционально) React/SPA build внутри Docker
# По умолчанию выключено: строки ниже закомментированы.
#
# Как включить:
#   1) Раскомментируйте блок FROM node... AS ui
#   2) Закомментируйте строку "COPY static /app/static" в runtime
#   3) Раскомментируйте строку "COPY --from=ui ..." в runtime
#   4) Убедитесь что есть папка ./web с package.json и npm run build
#
# ARG NODE_VERSION=22-alpine  # UNCOMMENT FOR REACT
# FROM node:${NODE_VERSION} AS ui  # UNCOMMENT FOR REACT
# WORKDIR /web  # UNCOMMENT FOR REACT
# COPY web/package*.json ./  # UNCOMMENT FOR REACT
# RUN npm ci  # UNCOMMENT FOR REACT
# COPY web .  # UNCOMMENT FOR REACT
# RUN npm run build  # UNCOMMENT FOR REACT


# ==========================
# Runtime
# ==========================
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app1 && adduser -S app1 -G app1 \
    && mkdir -p /data /app/static \
    && chown -R app1:app1 /data /app

WORKDIR /app

# Значения по умолчанию (перекроются из .env / docker-compose)
ENV DATA_DIR=/data \
    HTTP_ADDR=:8080 \
    STATIC_DIR=/app/static

COPY --from=builder /bin/onessa /app/onessa

# Текущая статика ("как сейчас")
COPY static /app/static
# COMMENT THIS LINE IF YOU SWITCH TO REACT BUILD

# React/SPA build ("как будет потом")
# COPY --from=ui /web/dist /app/static  # UNCOMMENT FOR REACT (adjust dist/build folder)

USER app1

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/onessa"]
