# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/tg-translate-bot ./main.go

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/tg-translate-bot /tg-translate-bot

USER nonroot:nonroot
ENTRYPOINT ["/tg-translate-bot"]
