# syntax=docker/dockerfile:1.7

FROM golang:1.21-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/skills-hub .

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

ENV PORT=8080 \
    DB_PATH=/app/data/skills.db \
    REDIS_URL=redis:6379 \
    COOKIE_SECURE=false \
    TRUST_PROXY_HEADERS=false \
    PLATFORM_ADMIN_EMAILS= \
    RESEND_API_KEY= \
    MAIL_FROM=noreply@example.com

COPY --from=builder /out/skills-hub /app/skills-hub
COPY --from=builder /src/templates /app/templates
COPY --from=builder /src/static /app/static

RUN mkdir -p /app/data /app/uploads

EXPOSE 8080

VOLUME ["/app/data", "/app/uploads"]

ENTRYPOINT ["/app/skills-hub"]
