# syntax=docker/dockerfile:1

FROM node:22-alpine AS web-builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY index.html postcss.config.mjs tsconfig.json vite.config.mts ./
COPY public ./public
COPY src ./src
RUN npm run build

FROM golang:1.26-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/lumen ./cmd/lumen

FROM alpine:3.21 AS runner
WORKDIR /app

RUN apk add --no-cache ca-certificates ffmpeg wget \
  && adduser -D -H -u 10001 lumen \
  && mkdir -p /app/data \
  && chown -R lumen:lumen /app

ENV PORT=3000
ENV LUMEN_DATA_DIR=/app/data

COPY --from=go-builder --chown=lumen:lumen /app/lumen /app/lumen

USER lumen
EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT:-3000}/api/health" >/dev/null || exit 1

CMD ["/app/lumen"]
