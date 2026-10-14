# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/bin/goshield ./cmd/server

# Production Stage (Ultra-minimal scratch image)
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/bin/goshield /usr/local/bin/goshield

EXPOSE 8080

USER 1000:1000

ENTRYPOINT ["/usr/local/bin/goshield"]
