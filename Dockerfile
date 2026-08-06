# syntax=docker/dockerfile:1

# ---------- dashboard 构建阶段 ----------
FROM node:22-bookworm AS dashboard-build
WORKDIR /app/cmd/mts-dashboard
COPY cmd/mts-dashboard/package.json cmd/mts-dashboard/package-lock.json ./
RUN npm ci --registry=https://registry.npmjs.org
COPY cmd/mts-dashboard/ ./
RUN npm run build

# ---------- mts-server 构建阶段 ----------
FROM golang:1.26 AS build
WORKDIR /src
ARG VERSION=dev
ARG COMMIT=unknown
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=dashboard-build /app/cmd/mts-server/dashboard-dist ./cmd/mts-server/dashboard-dist
RUN BUILT_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)" && \
    CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILT_AT}" \
    -o /out/mts-server ./cmd/mts-server

# ---------- 运行时阶段 ----------
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/mts-server /usr/local/bin/mts-server
COPY deploy/docker/mts-server.yaml /etc/mts-server/config.yaml
RUN mkdir -p /data && chown -R 1001:1001 /data
USER 1001:1001
WORKDIR /data
EXPOSE 8086 9096
VOLUME ["/data"]
ENTRYPOINT ["mts-server", "serve", "--config", "/etc/mts-server/config.yaml"]
