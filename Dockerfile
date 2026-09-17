# ─── Build Stage ─────────────────────────────────────────────────────────────
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY . .

# 依存解決とテスト、静的バイナリビルド
RUN go mod tidy && \
    CGO_ENABLED=0 go test -v ./... && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o tollgate cmd/server/main.go

# ─── Runtime Stage ───────────────────────────────────────────────────────────
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl wget

COPY --from=builder /app/tollgate /app/tollgate

ENV PORT=8000
ENV DYNAMODB_ENDPOINT=http://dynamodb:8000
ENV AWS_REGION=ap-northeast-1
ENV TABLE_NAME=ITCP_APIKeys

EXPOSE 8000

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD wget -q --spider http://localhost:8000/health/live || exit 1

CMD ["/app/tollgate"]
