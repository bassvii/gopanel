# --- Стадия 1: сборка фронтенда ---
FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- Стадия 2: сборка Go ---
FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/gopanel ./cmd/gopanel

# --- Стадия 3: финальный образ ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata unzip curl

# Xray — ставим в образ.
ARG XRAY_VERSION=latest
RUN if [ "$XRAY_VERSION" = "latest" ]; then \
        URL="https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip"; \
    else \
        URL="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/Xray-linux-64.zip"; \
    fi && \
    curl -fsSL -o /tmp/xray.zip "$URL" && \
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

EXPOSE 44559

VOLUME ["/var/lib/gopanel"]

ENV GOPANEL_DB_PATH=/var/lib/gopanel/gopanel.db

ENTRYPOINT ["/usr/local/bin/gopanel"]
CMD ["run"]

ENV GOPANEL_LISTEN=0.0.0.0
