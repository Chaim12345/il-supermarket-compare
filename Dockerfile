# syntax=docker/dockerfile:1

# ---- frontend ----
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- backend ----
FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
# modernc.org/sqlite is pure Go, so a static binary needs no libc.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ingest ./cmd/ingest \
 && mkdir -p /out/data

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/ingest /app/ingest
COPY --from=web /app/web/dist /app/web/dist
# Pre-create the data dir owned by the runtime user so a fresh volume is writable.
COPY --from=build --chown=nonroot:nonroot /out/data /data

ENV DB_PATH=/data/prices.db \
    STATIC_DIR=/app/web/dist \
    ADDR=:8080
VOLUME ["/data"]
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
