# ---- Stage 1: Next.js static export ----
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build
# → /web/out (static bundle served by Go in the final image)

# ---- Stage 2: Go binary (pure-Go SQLite, no CGO) ----
FROM golang:1.26-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/portofolio .

# ---- Stage 3: minimal runtime (single process) ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go /out/portofolio /app/portofolio
COPY --from=web /web/out /app/web/out
ENV PORT=8080 \
    DB_PATH=/app/data/portofolio.db \
    WEB_DIR=/app/web/out
EXPOSE 8080
VOLUME ["/app/data"]
CMD ["/app/portofolio"]
