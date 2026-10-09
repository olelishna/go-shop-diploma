FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download && go mod vendor

RUN go build ./cmd/gophermart

FROM alpine:latest
WORKDIR /app

COPY --from=builder /app .
