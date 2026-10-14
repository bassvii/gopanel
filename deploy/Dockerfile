# --- Стадия 1: сборка фронтенда ---
FROM node:24-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- Стадия 2: сборка Go ---
FROM golang:1.27.2-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/gopanel ./cmd/gopanel

# --- Стадия 3: финальный образ ---
FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc

RUN apk add --no-cache ca-certificates tzdata unzip curl

# Xray — фиксированная версия с проверкой SHA-256.
#
# ВАЖНО: значения дублируют deploy/versions.env. При обновлении
# править оба места. В перспективе — синхронизировать через
# build --build-arg (отдельная задача).
#
# Архитектура: amd64 (для arm64-образов — отдельная правка с TARGETARCH).
ARG XRAY_VERSION=v26.3.27
ARG XRAY_SHA256_AMD64=23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae
RUN curl -fsSL -o /tmp/xray.zip \
        "https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/Xray-linux-64.zip" && \
    echo "${XRAY_SHA256_AMD64}  /tmp/xray.zip" | sha256sum -c - && \
    unzip -q /tmp/xray.zip -d /tmp/xray && \
    mv /tmp/xray/xray /usr/local/bin/xray && \
    chmod +x /usr/local/bin/xray && \
    rm -rf /tmp/xray /tmp/xray.zip

COPY --from=build /out/gopanel /usr/local/bin/gopanel

# Каталог для БД.
RUN mkdir -p /var/lib/gopanel && \
    addgroup -S gopanel && \
    adduser -S -G gopanel -h /var/lib/gopanel gopanel && \
    chown -R gopanel:gopanel /var/lib/gopanel

USER gopanel

# HEALTHCHECK: проверяем, что панель отвечает на /login под base_path.
# 401 — это нормально (панель жива, требует логин). Не 0 exit —
# если порт не слушается или таймаут.
#
# base_path читается из файла, который панель пишет при старте
# (см. P2-13 / P2-7 аудита).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD sh -c 'PORT="${GOPANEL_PORT:-44559}"; BASE=$(cat /var/lib/gopanel/base_path 2>/dev/null || echo ""); wget -qO- "http://127.0.0.1:${PORT}${BASE}/login" >/dev/null 2>&1 || exit 1'

# Порт по умолчанию (44559) — только для справки.
# Реальный проброс портов делается через `-p` в docker run / compose.
# Listen-адрес и порт задаются переменными окружения GOPANEL_LISTEN
# и GOPANEL_PORT, которые образ не выставляет (см. пункт 10 аудита).
EXPOSE 44559

VOLUME ["/var/lib/gopanel"]

ENV GOPANEL_DB_PATH=/var/lib/gopanel/gopanel.db

ENTRYPOINT ["/usr/local/bin/gopanel"]
CMD ["run"]
